// ============================================================================
// 侧栏 —— AI 会话列表 + 普通群组列表 + 用户区（DSH 侧栏风格）
// ============================================================================
import { useState } from 'react';
import type { Group } from '../types/chat';
import styles from './Sidebar.module.scss';

interface SidebarProps {
  username: string;
  connected: boolean;
  statusMsg: string;
  totalUnread: number;
  aiGroups: Group[];
  normalGroups: Group[];
  selectedGroupId: string;
  activeGroup: string;
  unreadCounts: Record<string, number>;
  onSelectAI: (gid: string) => void;
  onSelectGroup: (gid: string) => void;
  onNewChat: () => void;
  onCreateGroup: (name: string) => void;
  onLogout: () => void;
}

const initials = (name: string): string => (name || '?').slice(0, 2).toUpperCase();

const Sidebar: React.FC<SidebarProps> = ({
  username, connected, statusMsg, totalUnread, aiGroups, normalGroups,
  selectedGroupId, activeGroup, unreadCounts, onSelectAI, onSelectGroup,
  onNewChat, onCreateGroup, onLogout,
}) => {
  const [newGroupName, setNewGroupName] = useState('');

  const submitGroup = () => {
    const name = newGroupName.trim();
    if (!name) return;
    onCreateGroup(name);
    setNewGroupName('');
  };

  return (
    <aside className={styles.sidebar}>
      {/* 用户区 */}
      <div className={styles.userRow}>
        <span className={styles.statusDot} data-online={connected} />
        <span className={styles.userName} title={username}>{username}</span>
        <button type='button' className={styles.logoutBtn} onClick={onLogout} title='退出登录'>
          <svg width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='currentColor'
            strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'>
            <path d='M9 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h4' />
            <polyline points='16 17 21 12 16 7' />
            <line x1='21' y1='12' x2='9' y2='12' />
          </svg>
        </button>
      </div>
      <div className={styles.statusLine}>{statusMsg}</div>
      {totalUnread > 0 && (
        <div className={styles.statusLine}>未读消息 {totalUnread > 999 ? '999+' : totalUnread}</div>
      )}

      {/* AI 会话 */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionTitle}>AI 会话</span>
          <button type='button' className={styles.newBtn} onClick={onNewChat} title='新建会话'>
            <svg width='14' height='14' viewBox='0 0 24 24' fill='none' stroke='currentColor'
              strokeWidth='2' strokeLinecap='round'>
              <line x1='12' y1='5' x2='12' y2='19' />
              <line x1='5' y1='12' x2='19' y2='12' />
            </svg>
          </button>
        </div>
        {aiGroups.length === 0 && (
          <div className={styles.empty}>还没有 AI 会话</div>
        )}
        {aiGroups.map((g) => (
          <div
            key={g.group_id}
            className={[styles.convItem, selectedGroupId === g.group_id ? styles.convActive : ''].join(' ')}
            onClick={() => onSelectAI(g.group_id)}
          >
            <span className={[styles.avatar, styles.avatarAI].join(' ')}>AI</span>
            <span className={styles.convName}>{g.group_name}</span>
            {!!unreadCounts[g.group_id] && (
              <span className={styles.badge}>
                {unreadCounts[g.group_id] > 99 ? '99+' : unreadCounts[g.group_id]}
              </span>
            )}
          </div>
        ))}
      </div>

      {/* 普通群组 */}
      <div className={styles.section}>
        <div className={styles.sectionHead}>
          <span className={styles.sectionTitle}>群组</span>
        </div>
        {normalGroups.map((g) => (
          <div
            key={g.group_id}
            className={[styles.convItem, activeGroup === g.group_id ? styles.convActive : ''].join(' ')}
            onClick={() => onSelectGroup(g.group_id)}
          >
            <span className={styles.avatar}>{initials(g.group_name)}</span>
            <span className={styles.convName}>{g.group_name}</span>
            {!!unreadCounts[g.group_id] && (
              <span className={styles.badge}>
                {unreadCounts[g.group_id] > 99 ? '99+' : unreadCounts[g.group_id]}
              </span>
            )}
          </div>
        ))}

        {/* 创建普通群组 */}
        <div className={styles.createRow}>
          <input
            className={styles.createInput}
            value={newGroupName}
            onChange={(e) => setNewGroupName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') submitGroup(); }}
            placeholder='新群组名称'
          />
          <button type='button' className={styles.createBtn} onClick={submitGroup}>创建</button>
        </div>
      </div>
    </aside>
  );
};

export default Sidebar;
