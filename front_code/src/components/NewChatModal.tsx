// ============================================================================
// 新会话弹窗 —— 只有一个 AI 会话入口（是否检索/调工具交给 agent loop 判断）
// ============================================================================
import styles from './NewChatModal.module.scss';

interface NewChatModalProps {
  open: boolean;
  onClose: () => void;
  onCreate: () => void;
}

const NewChatModal: React.FC<NewChatModalProps> = ({ open, onClose, onCreate }) => {
  if (!open) return null;

  return (
    <div className={styles.mask} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.title}>新建会话</div>
        <div className={styles.modeList}>
          <div className={styles.mode}>
            <span className={styles.modeTitle}>AI 聊天</span>
            <span className={styles.modeDesc}>
              由 agent loop 自行判断是否需要联网检索或调用工具
            </span>
          </div>
        </div>
        <div className={styles.actions}>
          <button type='button' className={styles.cancel} onClick={onClose}>取消</button>
          <button type='button' className={styles.create} onClick={onCreate}>创建</button>
        </div>
      </div>
    </div>
  );
};

export default NewChatModal;
