// ============================================================================
// 新会话弹窗 —— 选择 聊天 / 研究 模式后创建（DSH 弹窗风格）
// ============================================================================
import { useState } from 'react';
import styles from './NewChatModal.module.scss';

interface NewChatModalProps {
  open: boolean;
  onClose: () => void;
  onCreate: (mode: 'ai' | 'ai-research') => void;
}

const MODES = [
  { value: 'ai', title: '聊天模式', desc: '日常对话，快速回答' },
  { value: 'ai-research', title: '研究模式', desc: '联网检索、多轮审核、生成带引用的报告' },
] as const;

const NewChatModal: React.FC<NewChatModalProps> = ({ open, onClose, onCreate }) => {
  const [mode, setMode] = useState<'ai' | 'ai-research'>('ai');

  if (!open) return null;

  return (
    <div className={styles.mask} onClick={onClose}>
      <div className={styles.modal} onClick={(e) => e.stopPropagation()}>
        <div className={styles.title}>新建会话</div>
        <div className={styles.modeList}>
          {MODES.map((m) => (
            <button
              key={m.value}
              type='button'
              className={[styles.mode, mode === m.value ? styles.modeActive : ''].join(' ')}
              onClick={() => setMode(m.value)}
            >
              <span className={styles.modeTitle}>{m.title}</span>
              <span className={styles.modeDesc}>{m.desc}</span>
            </button>
          ))}
        </div>
        <div className={styles.actions}>
          <button type='button' className={styles.cancel} onClick={onClose}>取消</button>
          <button type='button' className={styles.create} onClick={() => onCreate(mode)}>创建</button>
        </div>
      </div>
    </div>
  );
};

export default NewChatModal;
