import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { chatApi } from '../api/chat';
import type { Group } from '../api/chat';
import styles from './ChatPage.module.scss';

const WS_BASE = 'ws://127.0.0.1:8080/api/v1/ws';

interface WsMessage {
  type: string;
  conversation_id?: string;
  group_id?: string;
  sender?: string;
  content?: string;
  time?: number;
}

const ChatPage: React.FC = () => {
  const navigate = useNavigate();
  const wsRef = useRef<WebSocket | null>(null);
  const msgEndRef = useRef<HTMLDivElement>(null);

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

  // --- WebSocket ---
  const connectWS = useCallback(() => {
    // 如果已有活跃连接，不重复创建
    if (wsRef.current && (wsRef.current.readyState === WebSocket.OPEN || wsRef.current.readyState === WebSocket.CONNECTING)) {
      return;
    }
    // 清除旧连接的 onclose，防止 StrictMode cleanup 触发重连
    if (wsRef.current) wsRef.current.onclose = null;

    const ws = new WebSocket(`${WS_BASE}?token=${encodeURIComponent(token)}`);
    wsRef.current = ws;

    ws.onopen = () => {
      setConnected(true);
      setStatusMsg('online');
    };

    ws.onmessage = (ev) => {
      try {
        const msg: WsMessage = JSON.parse(ev.data);
        setMessages((prev) => [...prev, msg]);
      } catch { /* ignore malformed */ }
    };

    ws.onclose = () => {
      setConnected(false);
      setStatusMsg('disconnected — reconnecting in 2s');
      setTimeout(connectWS, 2000);
    };

    ws.onerror = (e) => console.error("ws error:", e);
  }, [token]);

  useEffect(() => {
    connectWS();
    return () => {
      if (wsRef.current) wsRef.current.onclose = null;
      wsRef.current?.close();
    };
  }, [connectWS]);

  // scroll to bottom on new message
  useEffect(() => { msgEndRef.current?.scrollIntoView({ behavior: 'smooth' }); }, [messages]);

  // --- 初始化 & 加载群组 ---
  useEffect(() => {
    if (!userId) return;
    chatApi.initUser(userId, username).catch(() => {});
    chatApi.getUserGroups(userId).then((res) => setGroups(res.groups || [])).catch(() => {});
  }, [userId, username]);

  // --- 操作 ---
  const handleCreateGroup = async () => {
    if (!newGroupName.trim()) return;
    try {
      const res = await chatApi.createGroup(newGroupName.trim(), userId);
      setGroups((prev) => [...prev, { group_id: res.group_id, group_name: res.group_name }]);
      setNewGroupName('');
      setStatusMsg(`group ${res.group_id} created`);
    } catch { setStatusMsg('create group failed'); }
  };

  const handleJoinGroup = async () => {
    if (!joinGroupID.trim()) return;
    try {
      await chatApi.joinGroup(joinGroupID.trim(), userId);
      const res = await chatApi.getUserGroups(userId);
      setGroups(res.groups || []);
      setJoinGroupID('');
      setStatusMsg(`joined ${joinGroupID}`);
    } catch { setStatusMsg('join group failed'); }
  };

  const handleSend = () => {
    if (!input.trim() || !activeGroup) return;
    const msg = {
      type: 'chat',
      content: {
        conversation_type: 'group',
        sender_id: userId,
        group_id: activeGroup,
        text: input.trim(),
        message_id: `${userId}-${Date.now()}`,
        message_type: 'text',
      },
    };
    wsRef.current?.send(JSON.stringify(msg));
    setInput('');
  };

  const filteredMessages = messages.filter(
    (m) => m.group_id === activeGroup || m.conversation_id === activeGroup,
  );

  // --- logout ---
  const handleLogout = () => {
    localStorage.clear();
    wsRef.current?.close();
    navigate('/login');
  };

  return (
    <div className={styles.wrapper}>
      {/* sidebar */}
      <aside className={styles.sidebar}>
        <div className={styles.sidebarHeader}>
          <span className={styles.statusDot} data-online={connected} />
          <span>{username}</span>
          <button className={styles.logoutBtn} onClick={handleLogout}>X</button>
        </div>
        <div className={styles.statusLine}>{statusMsg}</div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>GROUPS</div>
          {groups.map((g) => (
            <div
              key={g.group_id}
              className={`${styles.groupItem} ${activeGroup === g.group_id ? styles.active : ''}`}
              onClick={() => setActiveGroup(g.group_id)}
            >
              {g.group_name} ({g.group_id})
            </div>
          ))}
        </div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>CREATE GROUP</div>
          <input className={styles.smallInput} value={newGroupName} onChange={(e) => setNewGroupName(e.target.value)} placeholder="group name" />
          <button className={styles.actionBtn} onClick={handleCreateGroup}>create</button>
        </div>

        <div className={styles.section}>
          <div className={styles.sectionTitle}>JOIN GROUP</div>
          <input className={styles.smallInput} value={joinGroupID} onChange={(e) => setJoinGroupID(e.target.value)} placeholder="group id e.g. G1" />
          <button className={styles.actionBtn} onClick={handleJoinGroup}>join</button>
        </div>
      </aside>

      {/* main chat */}
      <main className={styles.chatArea}>
        {!activeGroup ? (
          <div className={styles.placeholder}>select a group to start chatting</div>
        ) : (
          <>
            <div className={styles.chatHeader}>{activeGroup}</div>
            <div className={styles.messageList}>
              {filteredMessages.map((m, i) => (
                <div key={i} className={`${styles.msg} ${m.sender === userId ? styles.msgMine : ''}`}>
                  <div className={styles.msgSender}>{m.sender || 'system'}</div>
                  <div className={styles.msgContent}>{m.content}</div>
                </div>
              ))}
              <div ref={msgEndRef} />
            </div>
            <div className={styles.inputRow}>
              <input
                className={styles.chatInput}
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSend()}
                placeholder="type a message..."
              />
              <button className={styles.sendBtn} onClick={handleSend}>send</button>
            </div>
          </>
        )}
      </main>
    </div>
  );
};

export default ChatPage;
