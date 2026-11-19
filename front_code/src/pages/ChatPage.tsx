// ============================================================================
// ChatPage —— AI-only 装配层：会话列表、消息流、WS 收发
// ============================================================================
import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group, WsMessage, SendPayload } from '../types/chat';
import { useChatSocket } from '../hooks/useChatSocket';
import Sidebar from '../components/Sidebar';
import MessageList from '../components/MessageList';
import type { StreamEntry } from '../components/MessageList';
import ChatInput from '../components/ChatInput';
import NewChatModal from '../components/NewChatModal';
import styles from './ChatPage.module.scss';

const STORAGE_KEY = 'chant_ai_conversations';

function loadConversations(): Group[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

const ChatPage: React.FC = () => {
  const navigate = useNavigate();

  const token = localStorage.getItem('access_token') || '';
  const userInfo = JSON.parse(localStorage.getItem('user_info') || '{}');
  const userId: string = String(userInfo.user_id || '');
  const username: string = userInfo.username || 'Unknown';

  const [groups, setGroups] = useState<Group[]>(loadConversations);
  const [selectedGroupId, setSelectedGroupId] = useState<string>('');
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [input, setInput] = useState('');
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [progressSteps, setProgressSteps] = useState<Record<string, string[]>>({});
  const [streams, setStreams] = useState<Record<string, StreamEntry>>({});
  const [newChatOpen, setNewChatOpen] = useState(false);

  const currentGidRef = useRef<string>('');
  const seenMessageIdsRef = useRef<Set<string>>(new Set());

  const currentGid = selectedGroupId;
  const currentGroup = groups.find((g) => g.group_id === currentGid);
  const totalUnread = Object.values(unreadCounts).reduce((sum, n) => sum + n, 0);
  const filteredMessages = currentGid ? messages.filter((m) => m.group_id === currentGid) : [];

  // 本地保存 AI 会话列表；存储服务接入后替换为后端列表
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(groups));
  }, [groups]);

  useEffect(() => {
    currentGidRef.current = currentGid;
  }, [currentGid]);

  // ── WS 消息统一入口 ──
  const handleWsMessage = useCallback((msg: WsMessage) => {
    const convId = msg.conversation_id || msg.group_id || '';
    const normalized: WsMessage = { ...msg, group_id: convId };

    if (msg.sender === userId) return;

    // delta 分块：累积到流式条目，不进消息流、不计数未读
    if (msg.kind && msg.message_id) {
      const mid = msg.message_id;
      setStreams((prev) => {
        const cur = prev[mid] || {
          group_id: convId,
          sender: msg.sender || 'ai-assistant',
          reasoning: '',
          content: '',
          steps: [],
          done: false,
          time: msg.time,
          reply_to_msg_id: msg.reply_to_msg_id,
        };
        const next: StreamEntry = { ...cur, group_id: convId, steps: [...cur.steps] };
        if (msg.kind === 'thinking') next.reasoning += msg.content || '';
        else if (msg.kind === 'progress') {
          if (msg.content && !next.steps.includes(msg.content)) next.steps.push(msg.content);
        }
        else if (msg.kind === 'content') next.content += msg.content || '';
        else if (msg.kind === 'done') next.done = true;
        return { ...prev, [mid]: next };
      });
      return;
    }

    // 旧协议进度消息：挂靠到原始输入
    if (msg.metadata?.kind === 'progress' && msg.reply_to_msg_id) {
      const key = msg.reply_to_msg_id;
      setProgressSteps((prev) => {
        const texts = prev[key] || [];
        if (msg.content && texts.includes(msg.content)) return prev;
        return { ...prev, [key]: [...texts, msg.content || ''] };
      });
      return;
    }

    // 最终回复 / 错误到达：清除对应输入的进度点
    if (msg.reply_to_msg_id) {
      setProgressSteps((prev) => {
        if (!(msg.reply_to_msg_id! in prev)) return prev;
        const next = { ...prev };
        delete next[msg.reply_to_msg_id!];
        return next;
      });
    }

    // 按 message_id 去重，防 Kafka 重投
    if (msg.message_id) {
      if (seenMessageIdsRef.current.has(msg.message_id)) return;
      seenMessageIdsRef.current.add(msg.message_id);
    }
    // 最终整块消息到达：以服务端全文为准，清掉对应流式条目
    if (msg.message_id) {
      setStreams((prev) => {
        const next = { ...prev };
        let changed = false;
        for (const [id, s] of Object.entries(prev)) {
          if (id === msg.message_id
            || (msg.reply_to_msg_id && s.reply_to_msg_id === msg.reply_to_msg_id)) {
            delete next[id];
            changed = true;
          }
        }
        return changed ? next : prev;
      });
    }
    setMessages((prev) => [...prev, normalized]);

    // 非当前会话计入未读角标
    if (convId && convId !== currentGidRef.current) {
      setUnreadCounts((prev) => ({
        ...prev,
        [convId]: (prev[convId] || 0) + 1,
      }));
    }
  }, [userId]);

  const { connected, statusMsg, send } = useChatSocket({ token, onMessage: handleWsMessage });

  // 初始化用户资料（gateway 代理到 auth_service）
  useEffect(() => {
    if (!userId) return;
    chatApi.initUser(userId).catch(() => {});
  }, [userId]);

  // ── 新建 AI 会话（gateway 生成 conversation_id，第一条消息时才创建记录）──
  const handleCreateChat = async () => {
    try {
      const res = await chatApi.createConversation();
      const conversation: Group = {
        group_id: res.conversation_id,
        group_name: '新聊天',
      };
      setGroups((prev) => [conversation, ...prev]);
      setSelectedGroupId(conversation.group_id);
      setMessages([]);
      setNewChatOpen(false);
    } catch { /* ignore */ }
  };

  // ── 发送消息 ──
  const handleSend = () => {
    const text = input.trim();
    if (!text || !currentGid || !connected) return;

    const msgTime = Date.now();
    const msgId = crypto.randomUUID();
    const payload: SendPayload = {
      type: 'chat',
      content: {
        text,
        message_id: msgId,
        message_type: 'text',
        conversation_id: currentGid,
      },
    };

    if (!send(payload)) return;

    setMessages((prev) => [...prev, {
      type: 'chat',
      sender: userId,
      content: text,
      time: msgTime,
      group_id: currentGid,
      message_id: msgId,
    }]);
    setInput('');
  };

  const openConversation = (gid: string) => {
    setSelectedGroupId(gid);
    setUnreadCounts((prev) => (prev[gid] ? { ...prev, [gid]: 0 } : prev));
  };

  const handleLogout = () => {
    localStorage.clear();
    navigate('/login');
  };

  return (
    <div className={styles.wrapper}>
      <Sidebar
        username={username}
        connected={connected}
        statusMsg={statusMsg}
        totalUnread={totalUnread}
        aiGroups={groups}
        selectedGroupId={selectedGroupId}
        unreadCounts={unreadCounts}
        onSelectAI={openConversation}
        onNewChat={() => setNewChatOpen(true)}
        onLogout={handleLogout}
      />

      <main className={styles.main}>
        {!currentGid ? (
          <div className={styles.placeholder}>
            <div className={styles.placeholderTitle}>Chant</div>
            <div className={styles.placeholderDesc}>新建一个 AI 会话，开始聊天</div>
          </div>
        ) : (
          <>
            <header className={styles.header}>
              <span className={styles.headerTitle}>
                🤖 {currentGroup?.group_name || 'AI 会话'}
              </span>
            </header>

            <MessageList
              messages={filteredMessages}
              userId={userId}
              progressSteps={progressSteps}
              streams={streams}
              hasMoreHistory={false}
              loadingHistory={false}
              onLoadMore={() => {}}
            />

            <ChatInput
              value={input}
              onChange={setInput}
              onSend={handleSend}
            />
          </>
        )}
      </main>

      <NewChatModal
        open={newChatOpen}
        onClose={() => setNewChatOpen(false)}
        onCreate={handleCreateChat}
      />
    </div>
  );
};

export default ChatPage;
