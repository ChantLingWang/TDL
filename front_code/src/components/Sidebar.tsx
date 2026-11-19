// ============================================================================
// 侧栏 —— AI 会话列表 + 用户区（AI-only）
// ============================================================================
import type { Group } from '../types/chat';
import styles from './Sidebar.module.scss';

interface SidebarProps {
  username: string;
  connected: boolean;
  statusMsg: string;
  totalUnread: number;
  aiGroups: Group[];
  selectedGroupId: string;
  unreadCounts: Record<string, number>;
  onSelectAI: (gid: string) => void;
  onNewChat: () => void;
  onLogout: () => void;
}

const Sidebar: React.FC<SidebarProps> = ({
  username, connected, statusMsg, totalUnread, aiGroups,
  selectedGroupId, unreadCounts, onSelectAI, onNewChat, onLogout,
}) => {
  return (
    <aside className={styles.sidebar}>
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
    </aside>
  );
};

export default Sidebar;
