import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group } from '../api/chat';
import styles from './ChatPage.module.scss';

const WS_BASE = import.meta.env.VITE_WS_URL;

interface WsMessage {
  type: string;
  conversation_id?: string;
  group_id?: string;
  sender?: string;
  content?: string;
  time?: number;
}

// ---- 工具函数 ----
const fmtTime = (ts?: number): string => {
  if (!ts) return '';
  const d = new Date(ts);
  const h = d.getHours().toString().padStart(2, '0');
  const m = d.getMinutes().toString().padStart(2, '0');
  return `${h}:${m}`;
};

const initials = (name: string): string =>
  name.slice(0, 2).toUpperCase();

// ---- 自动增长 textarea ----
const autoGrow = (el: HTMLTextAreaElement) => {
  el.style.height = 'auto';
  el.style.height = Math.min(el.scrollHeight, 120) + 'px';
};

// ================================================================
const ChatPage: React.FC = () => {
  const navigate = useNavigate();
  const wsRef = useRef<WebSocket | null>(null);
  const msgEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);

  const token = localStorage.getItem('access_token') || '';
  const userInfo = JSON.parse(localStorage.getItem('user_info') || '{}');
  const userId: string = userInfo.user_id || '';
  const username: string = userInfo.username || 'Unknown';

  const [connected, setConnected] = useState(false);
  const [groups, setGroups] = useState<Group[]>([]);
  const [activeGroup, setActiveGroup] = useState<string>('');
  const [messages, setMessages] = useState<WsMessage[]>([]);
  const [input, setInput] = useState('');
  const [newGroupName, setNewGroupName] = useState('');
  const [joinGroupID, setJoinGroupID] = useState('');
  const [statusMsg, setStatusMsg] = useState('connecting...');

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

  useEffect(() => {
    if (!userId) return;
    chatApi.initUser(userId).catch(() => {});
    chatApi.getUserGroups(userId).then((res) => setGroups(res.groups || [])).catch(() => {});
  }, [userId, username]);

  // ---- 操作 ----
  const handleCreateGroup = async () => {
    if (!newGroupName.trim()) return;
    try {
      const res = await chatApi.createGroup(newGroupName.trim(), userId);
      setGroups((prev) => [...prev, { group_id: res.group_id, group_name: res.group_name }]);
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

  const handleSend = () => {
    const text = input.trim();
    if (!text || !activeGroup) return;
    const isAI = activeGroup === 'ai-assistant';
    const msgId = `${userId}-${Date.now()}`;
    const base = {
      type: 'chat',
      content: {
        sender_id: String(userId),
        text,
        message_id: msgId,
        message_type: 'text',
      },
    };
    const msg = isAI
      ? { ...base, content: { ...base.content, conversation_type: 'group', group_id: 'ai-assistant' } }
      : { ...base, content: { ...base.content, conversation_type: 'group', group_id: activeGroup } };
    // Optimistically show user message
    setMessages((prev) => [...prev, {
      type: 'chat',
      sender: username,
      content: text,
      time: Date.now(),
      conversation_id: activeGroup,
    }]);
    wsRef.current?.send(JSON.stringify(msg));
    setInput('');
    if (inputRef.current) { inputRef.current.style.height = 'auto'; }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const filteredMessages = activeGroup === 'ai-assistant'
    ? messages.filter(m => m.sender === 'ai-assistant' || m.conversation_id === 'ai-assistant' || (m.type === 'private_chat' && m.sender === 'ai-assistant'))
    : messages.filter(m => m.group_id === activeGroup || m.conversation_id === activeGroup);

  // 连续消息分组：同发送者、间隔 < 3 分钟计为一组
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

  return (
    <div className={styles.wrapper}>
      {/* ---- sidebar ---- */}
      <aside className={styles.sidebar}>
        <div className={styles.sidebarHeaderRow}>
          <span className={styles.statusDot} data-online={connected} />
          <span className={styles.sidebarUser}>{username}</span>
          <button className={styles.logoutBtn} onClick={() => { localStorage.clear(); wsRef.current?.close(); navigate('/login'); }}>X</button>
        </div>
        <div className={styles.statusLine}>{statusMsg}</div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>CHAT</div>
          <div className={`${styles.convItem} ${activeGroup === 'ai-assistant' ? styles.convActive : ''}`}
               onClick={() => setActiveGroup('ai-assistant')}>
            <span className={styles.avatar}>AI</span>
            <span>AI Assistant</span>
          </div>
        </div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>GROUPS</div>
          {groups.map(g => (
            <div key={g.group_id}
                 className={`${styles.convItem} ${activeGroup === g.group_id ? styles.convActive : ''}`}
                 onClick={() => setActiveGroup(g.group_id)}>
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

      {/* ---- 聊天区 ---- */}
      <main className={styles.chatArea}>
        {!activeGroup ? (
          <div className={styles.placeholder}>select a conversation to start chatting</div>
        ) : (
          <>
            <div className={styles.chatHeader}>
              {activeGroup === 'ai-assistant' ? '🤖 AI Assistant' : activeGroup}
            </div>
            <div className={styles.messageList}>
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
                placeholder={`Message ${activeGroup === 'ai-assistant' ? 'AI Assistant' : activeGroup}`}
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
