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
  const [joinGroupID, setJoinGroupID] = useState('');
  const [statusMsg, setStatusMsg] = useState('connecting...');
  const [loadingHistory, setLoadingHistory] = useState(false);
  const [hasMoreHistory, setHasMoreHistory] = useState(true);

  // 派生：AI 群组和普通群组
  const aiGroups = groups.filter(g => g.group_type === 'ai');
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
        setMessages((prev) => [...prev, msg]);
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
    }).catch(() => {});
  }, [userId]);

  // 切换会话时加载历史消息
  useEffect(() => {
    const gid = selectedGroupId || activeGroup;
    if (!gid) return;
    setMessages([]);
    chatApi.getHistory(gid, 50).then((res) => {
      const history: WsMessage[] = (res.messages || []).map((m: any) => ({
        type: 'chat',
        sender: m.sender_id,
        content: m.content,
        time: typeof m.timestamp === 'number' ? m.timestamp : Date.parse(m.timestamp),
        group_id: m.group_id || gid,
      }));
      setMessages(history);
    }).catch(() => {});
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
      }));
      more.reverse();
      setMessages((prev) => [...more, ...prev]);
      setHasMoreHistory(more.length === 50);
    } catch { } finally { setLoadingHistory(false); }
  };

  // ---- 新 AI 会话
  // ---- 新 AI 会话 = 创建 AI 群组 ----
  const handleNewChat = async () => {
    try {
      const group = await chatApi.createGroup('新聊天', userId, 'ai');
      setGroups((prev) => [group, ...prev]);
      setSelectedGroupId(group.group_id);
      setActiveGroup('');
      setMessages([]);
    } catch { /* ignore */ }
  };

  // ---- 创建/加入普通群组 ----
  const handleCreateGroup = async () => {
    if (!newGroupName.trim()) return;
    try {
      const group = await chatApi.createGroup(newGroupName.trim(), userId);
      setGroups((prev) => [...prev, group]);
      setNewGroupName('');
    } catch { /* ignore */ }
  };

  const handleJoinGroup = async () => {
    if (!joinGroupID.trim()) return;
    try {
      await chatApi.joinGroup(joinGroupID.trim(), userId);
      const res = await chatApi.getUserGroups(userId);
      setGroups(res.groups || []);
      setJoinGroupID('');
    } catch { /* ignore */ }
  };

  // ---- 发送消息 ----
  const handleSend = () => {
    const text = input.trim();
    if (!text) return;
    const gid = selectedGroupId || activeGroup;
    if (!gid) return;

    const msgTime = Date.now();
    const msgId = `${userId}-${msgTime}`;
    const selGroup = groups.find(g => g.group_id === gid);
    const convType = selGroup?.group_type === 'ai' ? 'ai' : 'group';

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
      type: 'chat', sender: username, content: text,
      time: msgTime, group_id: gid,
    }]);

    wsRef.current?.send(JSON.stringify(payload));
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

  const currentGroupName = groups.find(g => g.group_id === currentGid)?.group_name || '';

  return (
    <div className={styles.wrapper}>
      <aside className={styles.sidebar}>
        <div className={styles.sidebarHeaderRow}>
          <span className={styles.statusDot} data-online={connected} />
          <span className={styles.sidebarUser}>{username}</span>
          <button className={styles.logoutBtn} onClick={() => { localStorage.clear(); wsRef.current?.close(); navigate('/login'); }}>X</button>
        </div>
        <div className={styles.statusLine}>{statusMsg}</div>

        {/* AI 会话 = group_type === 'ai' */}
        <div className={styles.section}>
          <div className={styles.sectionTitle}>AI CHATS</div>
          <button className={styles.actionBtn} onClick={handleNewChat}>+ 新聊天</button>
          {aiGroups.map(g => (
            <div key={g.group_id}
                 className={`${styles.convItem} ${selectedGroupId === g.group_id ? styles.convActive : ''}`}
                 onClick={() => { setSelectedGroupId(g.group_id); setActiveGroup(''); }}>
              <span className={styles.avatar}>AI</span>
              <span>{g.group_name}</span>
            </div>
          ))}
        </div>

        {/* 普通群组 = group_type !== 'ai' */}
        <div className={styles.section}>
          <div className={styles.sectionTitle}>GROUPS</div>
          {normalGroups.map(g => (
            <div key={g.group_id}
                 className={`${styles.convItem} ${activeGroup === g.group_id ? styles.convActive : ''}`}
                 onClick={() => { setActiveGroup(g.group_id); setSelectedGroupId(''); }}>
              <span className={styles.avatar}>{initials(g.group_name)}</span>
              <span>{g.group_name}</span>
            </div>
          ))}
        </div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>NEW GROUP</div>
          <input className={styles.smallInput} value={newGroupName} onChange={e => setNewGroupName(e.target.value)} placeholder="group name" />
          <button className={styles.actionBtn} onClick={handleCreateGroup}>create</button>
          <input className={styles.smallInput} value={joinGroupID} onChange={e => setJoinGroupID(e.target.value)} placeholder="join by id e.g. G1" />
          <button className={styles.actionBtn} onClick={handleJoinGroup}>join</button>
        </div>
      </aside>

      <main className={styles.chatArea}>
        {!currentGid ? (
          <div className={styles.placeholder}>select a conversation to start chatting</div>
        ) : (
          <>
            <div className={styles.chatHeader}>
              {selectedGroupId ? `🤖 ${currentGroupName}` : currentGroupName || activeGroup}
            </div>
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
              {grouped.map((group, gi) => (
                <div key={gi} className={`${styles.msgGroup} ${group[0]?.sender === userId ? styles.msgGroupMine : ''}`}>
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
              ))}
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
              <button className={styles.sendBtn} onClick={handleSend} disabled={!input.trim()}>
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
