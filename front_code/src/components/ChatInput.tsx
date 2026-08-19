// ============================================================================
// 输入区 —— 自动增高 textarea + 发送按钮（DSH 输入区风格）
// ============================================================================
import { useRef } from 'react';
import styles from './ChatInput.module.scss';

interface ChatInputProps {
  value: string;
  onChange: (v: string) => void;
  onSend: () => void;
  placeholder?: string;
  /** 是否高亮发送按钮（研究模式） */
  accent?: boolean;
}

const ChatInput: React.FC<ChatInputProps> = ({
  value, onChange, onSend, placeholder = '输入消息…', accent,
}) => {
  const taRef = useRef<HTMLTextAreaElement>(null);

  const autoGrow = (el: HTMLTextAreaElement) => {
    el.style.height = 'auto';
    el.style.height = Math.min(el.scrollHeight, 140) + 'px';
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSend();
    }
  };

  const canSend = value.trim().length > 0;

  return (
    <div className={styles.dock}>
      <div className={styles.box}>
        <textarea
          ref={taRef}
          className={styles.input}
          value={value}
          onChange={(e) => { onChange(e.target.value); autoGrow(e.target); }}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          rows={1}
        />
        <button
          className={[styles.send, accent ? styles.sendAccent : ''].join(' ')}
          onClick={onSend}
          disabled={!canSend}
          title='发送（Enter）'
        >
          <svg width='16' height='16' viewBox='0 0 24 24' fill='none' stroke='currentColor'
            strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'>
            <line x1='22' y1='2' x2='11' y2='13' />
            <polygon points='22 2 15 22 11 13 2 9 22 2' />
          </svg>
        </button>
      </div>
    </div>
  );
};

export default ChatInput;
