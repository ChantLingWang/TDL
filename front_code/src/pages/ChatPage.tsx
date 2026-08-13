import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group } from '../api/chat';
import styles from './ChatPage.module.scss';

const WS_BASE = import.meta.env.VITE_WS_URL;

interface WsMessage {
  type: string;
  group_id?: string;
  sender?: string;
  content?: string;
  time?: number;
  message_id?: string;
  reply_to_msg_id?: string;
  metadata?: Record<string, string>;
}

const fmtTime = (ts?: number): string => {
  if (!ts) return '';
  const d = new Date(ts);
  return `${d.getHours().toString().padStart(2, '0')}:${d.getMinutes().toString().padStart(2, '0')}`;
};

const initials = (name: string): string => (name || '?').slice(0, 2).toUpperCase();

const autoGrow = (el: HTMLTextAreaElement) => {
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 120) + 'px';
};

const ChatPage: React.FC = () => {
  const navigate = useNavigate();
  const wsRef = useRef<WebSocket | null>(null);
  const msgEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const token = localStorage.getItem('access_token') || '';
  const userInfo = JSON.parse(localStorage.getItem('user_info') || '{}');
  const userId: string = String(userInfo.user_id || '');
  const username: string = userInfo.username || 'Unknown';

  const [connected, setConnected] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>('');
  const [selectedGroupId, setSelectedGroupId] = useState<string>('');
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [input, setInput] = useState('');
  const [newGroupName, setNewGroupName] = useState('');
  const [memberPanelOpen, setMemberPanelOpen] = useState(false);
  const [members, setMembers] = useState<string[]>([]);
  const [memberInput, setMemberInput] = useState('');
  const [memberStatus, setMemberStatus] = useState('');
  const [unreadCounts, setUnreadCounts] = useState<Record<string, number>>({});
  const [progressSteps, setProgressSteps] = useState<Record<string, string[]>>({});
  const [statusMsg, setStatusMsg] = useState('connecting...');
  const [agentMode, setAgentMode] = useState(false);
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(true);
  const currentGidRef = useRef<string>('');
  const seenMessageIdsRef = useRef<Set<string>>(new Set());

  // 派生：AI 群组和普通群组
  const aiGroups = groups.filter(g => g.group_type === 'ai' || g.group_type === 'ai-research');
  const normalGroups = groups.filter(g => g.group_type !== 'ai');

  // ---- WebSocket ----
  const connectWS = useCallback(() => {
    if (wsRef.current?.readyState === WebSocket.OPEN ||
        wsRef.current?.readyState === WebSocket.CONNECTING) return;
    if (wsRef.current) wsRef.current.onclose = null;

    const ws = new WebSocket(`${WS_BASE}?token=${encodeURIComponent(token)}`);
    wsRef.current = ws;

    ws.onopen = () => { setConnected(true); setStatusMsg('online'); };
    ws.onmessage = (ev) => {
      try {
        const msg: WsMessage = JSON.parse(ev.data);
        if (msg.sender === userId) return; // 跳过自己发的消息（本地已渲染）

        // 进度消息：不进入消息流，转为挂靠原始输入的进度步骤
        if (msg.metadata?.kind === 'progress' && msg.reply_to_msg_id) {
          setProgressSteps((prev) => {
            const key = msg.reply_to_msg_id!;
            const texts = prev[key] || [];
            if (msg.content && texts.includes(msg.content)) return prev;
            return { ...prev, [key]: [...texts, msg.content || ''] };
          });
          return;
        }

        // 最终回复 / 错误 / 审稿到达：清除对应输入的进度点
        if (msg.reply_to_msg_id) {
          setProgressSteps((prev) => {
            if (!(msg.reply_to_msg_id! in prev)) return prev;
            const next = { ...prev };
            delete next[msg.reply_to_msg_id!];
            return next;
          });
        }

        // 按 message_id 去重，防止 Kafka 重投导致重复显示
        if (msg.message_id) {
          if (seenMessageIdsRef.current.has(msg.message_id)) return;
          seenMessageIdsRef.current.add(msg.message_id);
        }
        setMessages((prev) => [...prev, msg]);
        // 非当前会话的消息计入未读角标
        if (msg.group_id && msg.group_id !== currentGidRef.current) {
          setUnreadCounts((prev) => ({
            ...prev,
            [msg.group_id!]: (prev[msg.group_id!] || 0) + 1,
          }));
        }
      } catch { /* ignore */ }
    };
    ws.onclose = () => {
      setConnected(false);
      setStatusMsg('disconnected — reconnecting in 2s');
      setTimeout(connectWS, 2000);
    };
    ws.onerror = () => {};
  }, [token]);

  useEffect(() => {
    connectWS();
    return () => { if (wsRef.current) { wsRef.current.onclose = null; wsRef.current.close(); } };
  }, [connectWS]);

  useEffect(() => { msgEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  // 保持 WS 回调里的当前会话 ID 为最新值，避免闭包过期
  useEffect(() => {
    currentGidRef.current = selectedGroupId || activeGroup;
  }, [selectedGroupId, activeGroup]);

  // ---- 初始化：加载所有群组（含 AI 群）----
  useEffect(() => {
    if (!userId) return;
    chatApi.initUser(userId).catch(() => {});
    chatApi.getUserGroups(userId).then((res) => {
      const list = res.groups || [];
      setGroups(list);
      // 默认选中第一个 AI 群
      const firstAI = list.find(g => g.group_type === 'ai');
      if (firstAI) setSelectedGroupId(firstAI.group_id);

      // 拉取离线期间的未读计数；默认打开的会话视为已读
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

  // 切换会话时加载历史消息
  useEffect(() => {
    const gid = selectedGroupId || activeGroup;
    if (!gid) return;
    setMessages([]);
    chatApi.getHistory(gid, 0, 50).then((res) => {
      const history: WsMessage[] = (res.messages || []).map((m: any) => ({
        type: 'chat',
        sender: m.sender_id,
        content: m.content,
        time: typeof m.timestamp === 'number' ? m.timestamp : Date.parse(m.timestamp),
        group_id: m.group_id || gid,
        message_id: m.message_id,
      }));
      // 后端返回最新在前，前端需要旧 -> 新
      setMessages([...history].reverse());
    }).catch(() => {});
  }, [selectedGroupId, activeGroup]);

  // 切换会话时收起成员管理面板
  useEffect(() => {
    setMemberPanelOpen(false);
    setMembers([]);
    setMemberInput('');
    setMemberStatus('');
  }, [selectedGroupId, activeGroup]);

  // ---- 加载更多历史 ----
  const handleLoadMore = async () => {
    const gid = selectedGroupId || activeGroup;
    if (!gid || messages.length === 0) return;
    const earliestTime = messages[0]?.time;
    if (!earliestTime) return;
    const cursor = Math.floor(earliestTime / 1000);
    setLoadingHistory(true);
    try {
      const res = await chatApi.getHistory(gid, cursor, 50);
      const more: WsMessage[] = (res.messages || []).map((m: any) => ({
        type: 'chat',
        sender: m.sender_id,
        content: m.content,
        time: typeof m.timestamp === 'number' ? m.timestamp : Date.parse(m.timestamp),
        group_id: m.group_id || gid,
        message_id: m.message_id,
      }));
      // more 为最新在前，转为旧 -> 新后整体前置
      more.reverse();
      setMessages((prev) => [...more, ...prev]);
      setHasMoreHistory(res.messages?.length === 50);
    } catch { } finally { setLoadingHistory(false); }
  };

  // ---- 新 AI 会话
  // ---- 新 AI 会话 = 创建 AI 群组 ----
  const handleNewChat = async () => {
    try {
      const gtype = agentMode ? 'ai-research' : 'ai';
      const group = await chatApi.createGroup('新聊天', userId, gtype);
      setGroups((prev) => [group, ...prev]);
      setSelectedGroupId(group.group_id);
      setActiveGroup('');
      setMessages([]);
    } catch { /* ignore */ }
  };

  // ---- 创建普通群组 ----
  const handleCreateGroup = async () => {
    if (!newGroupName.trim()) return;
    try {
      const group = await chatApi.createGroup(newGroupName.trim(), userId);
      setGroups((prev) => [...prev, group]);
      setNewGroupName('');
    } catch { /* ignore */ }
  };

  // ---- 发送消息 ----
  const handleSend = () => {
    const text = input.trim();
    if (!text) return;
    const gid = selectedGroupId || activeGroup;
    if (!gid) return;

    // 连接未就绪时不发送，避免“本地已显示但实际没发出”
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      setStatusMsg('连接已断开，消息未发送，请稍后重试');
      return;
    }

    const msgTime = Date.now();
    const msgId = `${userId}-${msgTime}`;
    const selGroup = groups.find(g => g.group_id === gid);
    const groupType = selGroup?.group_type || 'group';
    const isAI = groupType === 'ai' || groupType === 'ai-research';
    // ai-research 群固定走研究模式；普通群始终是 group，不受 Research 开关影响
    const convType = groupType === 'ai-research'
      ? 'ai-research'
      : (isAI && agentMode) ? 'ai-research' : (isAI ? 'ai' : 'group');

    const payload: any = {
      type: 'chat',
      content: {
        sender_id: userId,
        text,
        message_id: msgId,
        message_type: 'text',
        conversation_type: convType,
        group_id: gid,
      },
    };

    setMessages((prev) => [...prev, {
      type: 'chat', sender: userId, content: text,
      time: msgTime, group_id: gid, message_id: msgId,
    }]);

    ws.send(JSON.stringify(payload));
    setInput('');
    if (inputRef.current) { inputRef.current.style.height = 'auto'; }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  // ---- 消息过滤 ----
  const currentGid = selectedGroupId || activeGroup;
  const filteredMessages = currentGid
    ? messages.filter(m => m.group_id === currentGid)
    : [];

  const grouped = filteredMessages.reduce<WsMessage[][]>((acc, msg) => {
    const prevGroup = acc[acc.length - 1];
    if (prevGroup) {
      const prevMsg = prevGroup[prevGroup.length - 1];
      if (prevMsg.sender === msg.sender && msg.time && prevMsg.time && (msg.time - prevMsg.time) < 180_000) {
        prevGroup.push(msg);
        return acc;
      }
    }
    acc.push([msg]);
    return acc;
  }, []);

  const currentGroup = groups.find(g => g.group_id === currentGid);
  const currentGroupName = currentGroup?.group_name || '';
  const isOwner = !!currentGroup && currentGroup.create_by_user_id === userId;
  const isAIConversation = currentGroup?.group_type === 'ai' || currentGroup?.group_type === 'ai-research';
  const totalUnread = Object.values(unreadCounts).reduce((sum, n) => sum + n, 0);

  // ---- 打开会话即已读 ----
  const openAIConversation = (gid: string) => {
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

  // ---- 群主成员管理 ----
  const handleToggleMemberPanel = async () => {
    if (memberPanelOpen) {
      setMemberPanelOpen(false);
      return;
    }
    setMemberPanelOpen(true);
    setMemberStatus('');
    try {
      const res = await chatApi.getGroupMembers(currentGid);
      setMembers(res.members || []);
    } catch {
      setMembers([]);
    }
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

  return (
    <div className={styles.wrapper}>
      <aside className={styles.sidebar}>
        <div className={styles.sidebarHeaderRow}>
          <span className={styles.statusDot} data-online={connected} />
          <span className={styles.sidebarUser}>{username}</span>
          <button className={styles.logoutBtn} onClick={() => { localStorage.clear(); wsRef.current?.close(); navigate('/login'); }}>X</button>
        </div>
        <div className={styles.statusLine}>{statusMsg}</div>
        {totalUnread > 0 && (
          <div className={styles.statusLine}>
            未读消息 {totalUnread > 999 ? '999+' : totalUnread}
          </div>
        )}

        {/* AI 会话 = group_type === 'ai' */}
        <div className={styles.section}>
          <div className={styles.sectionTitle}>AI CHATS</div>
          <button className={styles.actionBtn} onClick={handleNewChat}>+ 新聊天</button>
          {aiGroups.map(g => (
            <div key={g.group_id}
                 className={`${styles.convItem} ${selectedGroupId === g.group_id ? styles.convActive : ''}`}
                 onClick={() => openAIConversation(g.group_id)}>
              <span className={styles.avatar}>AI</span>
              <span>{g.group_name}</span>
              {!!unreadCounts[g.group_id] && (
                <span className={styles.unreadBadge}>
                  {unreadCounts[g.group_id] > 99 ? '99+' : unreadCounts[g.group_id]}
                </span>
              )}
            </div>
          ))}
        </div>

        {/* 普通群组 = group_type !== 'ai' */}
        <div className={styles.section}>
          <div className={styles.sectionTitle}>GROUPS</div>
          {normalGroups.map(g => (
            <div key={g.group_id}
                 className={`${styles.convItem} ${activeGroup === g.group_id ? styles.convActive : ''}`}
                 onClick={() => openNormalGroup(g.group_id)}>
              <span className={styles.avatar}>{initials(g.group_name)}</span>
              <span>{g.group_name}</span>
              {!!unreadCounts[g.group_id] && (
                <span className={styles.unreadBadge}>
                  {unreadCounts[g.group_id] > 99 ? '99+' : unreadCounts[g.group_id]}
                </span>
              )}
            </div>
          ))}
        </div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>NEW GROUP</div>
          <input className={styles.smallInput} value={newGroupName} onChange={e => setNewGroupName(e.target.value)} placeholder="group name" />
          <button className={styles.actionBtn} onClick={handleCreateGroup}>create</button>
        </div>
      </aside>

      <main className={styles.chatArea}>
        {!currentGid ? (
          <div className={styles.placeholder}>select a conversation to start chatting</div>
        ) : (
          <>
            <div className={styles.chatHeader}>
              <span>{selectedGroupId ? `🤖 ${currentGroupName}` : currentGroupName || activeGroup}</span>
              {isOwner && (
                <button className={styles.manageBtn} onClick={handleToggleMemberPanel}>
                  {memberPanelOpen ? '收起' : '拉人'}
                </button>
              )}
              {isAIConversation && (
                <label className={styles.agentToggle} title={agentMode ? '研究模式开启' : '聊天模式'}>
                  <input type="checkbox" checked={agentMode} onChange={e => setAgentMode(e.target.checked)} />
                  <span className={styles.toggleSlider} />
                  <span className={styles.toggleLabel}>{agentMode ? 'Research' : 'Chat'}</span>
                </label>
              )}
            </div>
            {memberPanelOpen && (
              <div className={styles.memberPanel}>
                <div className={styles.memberPanelTitle}>群成员管理</div>
                <div className={styles.memberAddRow}>
                  <input
                    className={styles.smallInput}
                    value={memberInput}
                    onChange={e => setMemberInput(e.target.value)}
                    placeholder="输入用户 ID"
                  />
                  <button className={styles.actionBtn} onClick={handleAddMember}>拉入</button>
                </div>
                <div className={styles.memberList}>
                  {members.map(m => (
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
                        <div className={styles.messageList}>
              {hasMoreHistory && (
                <button
                  className={styles.actionBtn}
                  onClick={handleLoadMore}
                  disabled={loadingHistory}
                  style={{ display: 'block', margin: '0 auto 8px' }}
                >
                  {loadingHistory ? '加载中...' : '加载更多'}
                </button>
              )}
              {grouped.map((group, gi) => {
                const lastMsg = group[group.length - 1];
                const steps = lastMsg?.sender === userId
                  ? progressSteps[lastMsg.message_id || '']
                  : undefined;
                return (
                  <div key={gi}>
                    <div className={`${styles.msgGroup} ${group[0]?.sender === userId ? styles.msgGroupMine : ''}`}>
                      {group[0]?.sender !== userId && (
                        <div className={styles.msgAvatar}>{initials(group[0]?.sender || '?')}</div>
                      )}
                      <div className={styles.msgBubbles}>
                        {group.map((m, mi) => (
                          <div key={mi} className={styles.msgBubble}>
                            {mi === 0 && group[0]?.sender !== userId && (
                              <div className={styles.msgName}>{m.sender}</div>
                            )}
                            <div className={styles.msgText}>{m.content}</div>
                            {mi === group.length - 1 && (
                              <div className={styles.msgTime}>{fmtTime(m.time)}</div>
                            )}
                          </div>
                        ))}
                      </div>
                    </div>
                    {steps && steps.length > 0 && (
                      <div className={styles.progressTracker}>
                        <div className={styles.progressDots}>
                          {steps.map((text, si) => (
                            <span
                              key={si}
                              title={text}
                              className={`${styles.progressDot} ${
                                si === steps.length - 1
                                  ? styles.progressDotActive
                                  : styles.progressDotDone
                              }`}
                            />
                          ))}
                        </div>
                        <span className={styles.progressLabel}>{steps[steps.length - 1]}</span>
                      </div>
                    )}
                  </div>
                );
              })}
              <div ref={msgEndRef} />
            </div>

            <div className={styles.inputRow}>
              <textarea
                ref={inputRef}
                className={styles.chatInput}
                value={input}
                onChange={e => { setInput(e.target.value); autoGrow(e.target); }}
                onKeyDown={handleKeyDown}
                placeholder="输入消息..."
                rows={1}
              />
              <button className={`${styles.sendBtn} ${agentMode ? styles.sendBtnAgent : ''}`} onClick={handleSend} disabled={!input.trim()}>
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <line x1="22" y1="2" x2="11" y2="13"/><polygon points="22 2 15 22 11 13 2 9 22 2"/>
                </svg>
              </button>
            </div>
          </>
        )}
      </main>
    </div>
  );
};

export default ChatPage;
