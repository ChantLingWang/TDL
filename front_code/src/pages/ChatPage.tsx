// ============================================================================
// ChatPage —— 装配层：持有状态与数据流，渲染全部委托给组件
// 状态职责：群组/会话选择、消息流、未读、进度步骤、成员管理
// ============================================================================
import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group, WsMessage, SendPayload } from '../types/chat';
import { isAIGroup } from '../types/chat';
import { useChatSocket } from '../hooks/useChatSocket';
import Sidebar from '../components/Sidebar';
import MessageList from '../components/MessageList';
import type { StreamEntry } from '../components/MessageList';
import ChatInput from '../components/ChatInput';
import NewChatModal from '../components/NewChatModal';
import styles from './ChatPage.module.scss';

const ChatPage: React.FC = () => {
  const navigate = useNavigate();

  const token = localStorage.getItem('access_token') || '';
  const userInfo = JSON.parse(localStorage.getItem('user_info') || '{}');
  const userId: string = String(userInfo.user_id || '');
  const username: string = userInfo.username || 'Unknown';

  // ── 会话与消息状态 ──
  const [groups, setGroups] = useState<Group[]>([]);
  const [selectedGroupId, setSelectedGroupId] = useState<string>('');
  const [activeGroup, setActiveGroup] = useState<string>('');
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [input, setInput] = useState('');
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [progressSteps, setProgressSteps] = useState<Record<string, string[]>>({});
  const [streams, setStreams] = useState<Record<string, StreamEntry>>({});
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(true);
  const [agentMode, setAgentMode] = useState(false);
  const [newChatOpen, setNewChatOpen] = useState(false);

  // ── 成员管理状态 ──
  const [memberPanelOpen, setMemberPanelOpen] = useState(false);
  const [members, setMembers] = useState<string[]>([]);
  const [memberInput, setMemberInput] = useState('');
  const [memberStatus, setMemberStatus] = useState('');

  const currentGidRef = useRef<string>('');
  const seenMessageIdsRef = useRef<Set<string>>(new Set());

  const currentGid = selectedGroupId || activeGroup;
  const currentGroup = groups.find((g) => g.group_id === currentGid);
  const aiGroups = groups.filter((g) => isAIGroup(g));
  const normalGroups = groups.filter((g) => !isAIGroup(g));

  // ── WS 消息统一入口 ──
  const handleWsMessage = useCallback((msg: WsMessage) => {
    if (msg.sender === userId) return; // 自己发的已在本地渲染

    // delta 分块（kind 字段存在）：累积到流式条目，不进消息流、不计数未读
    if (msg.kind && msg.message_id) {
      const mid = msg.message_id;
      setStreams((prev) => {
        const cur = prev[mid] || {
          group_id: msg.group_id || '',
          sender: msg.sender || 'ai-assistant',
          reasoning: '',
          content: '',
          steps: [],
          done: false,
          time: msg.time,
          reply_to_msg_id: msg.reply_to_msg_id,
        };
        const next: StreamEntry = { ...cur, steps: [...cur.steps] };
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

    // 进度消息（旧协议）：不进入消息流，挂靠到原始输入
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
    // 最终整块消息到达：以服务端全文为准，清掉对应流式条目。
    // 按 message_id 清（正常最终回复），再按 reply_to_msg_id 兜底清（错误回复等异常收尾）
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
    setMessages((prev) => [...prev, msg]);

    // 非当前会话计入未读角标
    if (msg.group_id && msg.group_id !== currentGidRef.current) {
      setUnreadCounts((prev) => ({
        ...prev,
        [msg.group_id!]: (prev[msg.group_id!] || 0) + 1,
      }));
    }
  }, [userId]);

  const { connected, statusMsg, send } = useChatSocket({ token, onMessage: handleWsMessage });

  // 保持 WS 回调里的当前会话 ID 最新
  useEffect(() => {
    currentGidRef.current = currentGid;
  }, [currentGid]);

  // ── 初始化：加载群组与未读 ──
  useEffect(() => {
    if (!userId) return;
    chatApi.initUser(userId).catch(() => {});
    chatApi.getUserGroups(userId).then((res) => {
      const list = res.groups || [];
      setGroups(list);
      const firstAI = list.find((g) => g.group_type === 'ai');
      if (firstAI) setSelectedGroupId(firstAI.group_id);

      chatApi.getMessages().then((unreadRes) => {
        const counts = unreadRes.unread_counts || {};
        if (firstAI) {
          delete counts[firstAI.group_id];
          chatApi.markMessagesAsRead(firstAI.group_id).catch(() => {});
        }
        setUnreadCounts(counts);
      }).catch(() => {});
    }).catch(() => {});
  }, [userId]);

  // ── 切换会话：加载历史（旧 → 新）──
  useEffect(() => {
    if (!currentGid) return;
    setMessages([]);
    chatApi.getHistory(currentGid, 0, 50).then((res) => {
      const history: WsMessage[] = (res.messages || []).map((m: any) => ({
        type: 'chat',
        sender: m.sender_id,
        content: m.content,
        time: typeof m.timestamp === 'number' ? m.timestamp : Date.parse(m.timestamp),
        group_id: m.group_id || currentGid,
        message_id: m.message_id,
        reasoning: m.metadata?.reasoning,
      }));
      setMessages([...history].reverse());
    }).catch(() => {});
  }, [selectedGroupId, activeGroup]);

  // 切换会话时收起成员面板
  useEffect(() => {
    setMemberPanelOpen(false);
    setMembers([]);
    setMemberInput('');
    setMemberStatus('');
  }, [selectedGroupId, activeGroup]);

  // ── 加载更早历史 ──
  const handleLoadMore = async () => {
    if (!currentGid || messages.length === 0) return;
    const earliestTime = messages[0]?.time;
    if (!earliestTime) return;
    setLoadingHistory(true);
    try {
      const res = await chatApi.getHistory(currentGid, Math.floor(earliestTime / 1000), 50);
      const more: WsMessage[] = (res.messages || []).map((m: any) => ({
        type: 'chat',
        sender: m.sender_id,
        content: m.content,
        time: typeof m.timestamp === 'number' ? m.timestamp : Date.parse(m.timestamp),
        group_id: m.group_id || currentGid,
        message_id: m.message_id,
        reasoning: m.metadata?.reasoning,
      }));
      more.reverse();
      setMessages((prev) => [...more, ...prev]);
      setHasMoreHistory(res.messages?.length === 50);
    } catch { /* ignore */ } finally { setLoadingHistory(false); }
  };

  // ── 新建 AI 会话（弹窗确认模式后创建）──
  const handleCreateChat = async (mode: 'ai' | 'ai-research') => {
    setNewChatOpen(false);
    try {
      const group = await chatApi.createGroup('新聊天', userId, mode);
      setGroups((prev) => [group, ...prev]);
      setSelectedGroupId(group.group_id);
      setActiveGroup('');
      setMessages([]);
    } catch { /* ignore */ }
  };

  // ── 创建普通群组 ──
  const handleCreateGroup = async (name: string) => {
    try {
      const group = await chatApi.createGroup(name, userId);
      setGroups((prev) => [...prev, group]);
    } catch { /* ignore */ }
  };

  // ── 发送消息 ──
  const handleSend = () => {
    const text = input.trim();
    if (!text || !currentGid) return;
    if (!connected) return;

    const msgTime = Date.now();
    const msgId = userId + '-' + msgTime;
    const groupType = currentGroup?.group_type || 'group';
    const isAI = isAIGroup(currentGroup);
    const convType = groupType === 'ai-research'
      ? 'ai-research'
      : (isAI && agentMode) ? 'ai-research' : (isAI ? 'ai' : 'group');

    const payload: SendPayload = {
      type: 'chat',
      content: {
        sender_id: userId,
        text,
        message_id: msgId,
        message_type: 'text',
        conversation_type: convType,
        group_id: currentGid,
      },
    };

    const ok = send(payload);
    if (!ok) return; // 连接断开，不本地渲染

    setMessages((prev) => [...prev, {
      type: 'chat', sender: userId, content: text,
      time: msgTime, group_id: currentGid, message_id: msgId,
    }]);
    setInput('');
  };

  // ── 会话切换 / 已读 ──
  const openConversation = (gid: string) => {
    setSelectedGroupId(gid);
    setActiveGroup('');
    setUnreadCounts((prev) => (prev[gid] ? { ...prev, [gid]: 0 } : prev));
    chatApi.markMessagesAsRead(gid).catch(() => {});
  };

  const openNormalGroup = (gid: string) => {
    setActiveGroup(gid);
    setSelectedGroupId('');
    setUnreadCounts((prev) => (prev[gid] ? { ...prev, [gid]: 0 } : prev));
    chatApi.markMessagesAsRead(gid).catch(() => {});
  };

  // ── 成员管理 ──
  const handleToggleMemberPanel = async () => {
    if (memberPanelOpen) { setMemberPanelOpen(false); return; }
    setMemberPanelOpen(true);
    setMemberStatus('');
    try {
      const res = await chatApi.getGroupMembers(currentGid);
      setMembers(res.members || []);
    } catch { setMembers([]); }
  };

  const handleAddMember = async () => {
    const targetId = memberInput.trim();
    if (!targetId) return;
    setMemberStatus('');
    try {
      await chatApi.addGroupMember(currentGid, targetId);
      setMemberInput('');
      const res = await chatApi.getGroupMembers(currentGid);
      setMembers(res.members || []);
    } catch {
      setMemberStatus('添加失败，请确认用户 ID');
    }
  };

  const handleRemoveMember = async (targetId: string) => {
    setMemberStatus('');
    try {
      await chatApi.removeGroupMember(currentGid, targetId);
      setMembers((prev) => prev.filter((m) => m !== targetId));
    } catch {
      setMemberStatus('移除失败');
    }
  };

  const handleLogout = () => {
    localStorage.clear();
    navigate('/login');
  };

  const currentGroupName = currentGroup?.group_name || '';
  const isOwner = !!currentGroup && currentGroup.create_by_user_id === userId;
  const totalUnread = Object.values(unreadCounts).reduce((sum, n) => sum + n, 0);
  const filteredMessages = currentGid ? messages.filter((m) => m.group_id === currentGid) : [];

  return (
    <div className={styles.wrapper}>
      <Sidebar
        username={username}
        connected={connected}
        statusMsg={statusMsg}
        totalUnread={totalUnread}
        aiGroups={aiGroups}
        normalGroups={normalGroups}
        selectedGroupId={selectedGroupId}
        activeGroup={activeGroup}
        unreadCounts={unreadCounts}
        onSelectAI={openConversation}
        onSelectGroup={openNormalGroup}
        onNewChat={() => setNewChatOpen(true)}
        onCreateGroup={handleCreateGroup}
        onLogout={handleLogout}
      />

      <main className={styles.main}>
        {!currentGid ? (
          <div className={styles.placeholder}>
            <div className={styles.placeholderTitle}>Chant</div>
            <div className={styles.placeholderDesc}>选择或新建一个 AI 会话，开始聊天</div>
          </div>
        ) : (
          <>
            <header className={styles.header}>
              <span className={styles.headerTitle}>
                {selectedGroupId ? '🤖 ' + currentGroupName : currentGroupName || activeGroup}
              </span>
              <div className={styles.headerActions}>
                {isOwner && (
                  <button className={styles.manageBtn} onClick={handleToggleMemberPanel}>
                    {memberPanelOpen ? '收起' : '成员'}
                  </button>
                )}
                {isAIGroup(currentGroup) && (
                  <label className={styles.agentToggle} title={agentMode ? '研究模式开启' : '聊天模式'}>
                    <input type='checkbox' checked={agentMode} onChange={(e) => setAgentMode(e.target.checked)} />
                    <span className={styles.toggleSlider} />
                    <span className={styles.toggleLabel}>{agentMode ? 'Research' : 'Chat'}</span>
                  </label>
                )}
              </div>
            </header>

            {memberPanelOpen && (
              <div className={styles.memberPanel}>
                <div className={styles.memberPanelTitle}>群成员管理</div>
                <div className={styles.memberAddRow}>
                  <input
                    className={styles.memberInput}
                    value={memberInput}
                    onChange={(e) => setMemberInput(e.target.value)}
                    placeholder='输入用户 ID'
                  />
                  <button className={styles.memberBtn} onClick={handleAddMember}>拉入</button>
                </div>
                <div className={styles.memberList}>
                  {members.map((m) => (
                    <div key={m} className={styles.memberItem}>
                      <span>{m}</span>
                      {m !== userId && (
                        <button className={styles.memberRemove} onClick={() => handleRemoveMember(m)}>移除</button>
                      )}
                    </div>
                  ))}
                  {members.length === 0 && <span className={styles.memberEmpty}>暂无成员</span>}
                </div>
                {memberStatus && <div className={styles.memberStatus}>{memberStatus}</div>}
              </div>
            )}

            <MessageList
              messages={filteredMessages}
              userId={userId}
              progressSteps={progressSteps}
              streams={streams}
              hasMoreHistory={hasMoreHistory}
              loadingHistory={loadingHistory}
              onLoadMore={handleLoadMore}
            />

            <ChatInput
              value={input}
              onChange={setInput}
              onSend={handleSend}
              accent={agentMode}
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
