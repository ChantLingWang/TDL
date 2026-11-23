// ============================================================================
// ChatPage —— AI-only 装配层：会话列表、消息流、WS 收发
// ============================================================================
import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group, StepMessage, StepStream, WsMessage, SendPayload } from '../types/chat';
import { useChatSocket } from '../hooks/useChatSocket';
import Sidebar from '../components/Sidebar';
import MessageList from '../components/MessageList';
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
  const [streams, setStreams] = useState<Record<string, StepStream>>({});
  const [newChatOpen, setNewChatOpen] = useState(false);

  const currentGidRef = useRef<string>('');

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

  // ── WS 消息统一入口：只处理 AI step 事件 ──
  const handleWsMessage = useCallback((msg: WsMessage) => {
    if (msg.type !== 'ai_step') return;

    const stepMsg = msg as StepMessage;
    const mid = stepMsg.message_id;
    if (!mid) return;

    const convId = stepMsg.conversation_id || '';
    const attempt = stepMsg.attempt ?? 0;
    const round = stepMsg.round ?? 0;

    setStreams((prev) => {
      const cur = prev[mid];
      // 忽略旧 attempt / 旧 round 的迟到事件
      if (cur && (attempt < cur.attempt || round < cur.round)) return prev;
      const reset = !cur || attempt > cur.attempt || round > cur.round;
      const base: StepStream = reset
        ? {
            conversation_id: convId,
            reply_to_msg_id: stepMsg.reply_to_msg_id,
            message_id: mid,
            sender: stepMsg.sender,
            round,
            attempt,
            steps: [],
            done: false,
          }
        : cur;

      const steps = base.steps.filter((item) => item.step !== stepMsg.step);
      steps.push({
        kind: stepMsg.kind,
        step: stepMsg.step,
        payload: stepMsg.payload || {},
        time: stepMsg.time,
      });
      steps.sort((a, b) => a.step - b.step);

      return {
        ...prev,
        [mid]: {
          ...base,
          steps,
          done: base.done || stepMsg.kind === 'RESULT',
        },
      };
    });

    // 非当前会话的完成事件计入未读
    if (stepMsg.kind === 'RESULT' && convId && convId !== currentGidRef.current) {
      setUnreadCounts((prev) => ({
        ...prev,
        [convId]: (prev[convId] || 0) + 1,
      }));
    }
  }, []);

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
      setStreams({});
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
              conversationId={currentGid}
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
