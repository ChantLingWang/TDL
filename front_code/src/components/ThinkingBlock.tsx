// ============================================================================
// 思考块 —— DSH 式可折叠思考区域
// 阶段一：渲染后端 progress 步骤 + 历史消息的 reasoning 全文
// 阶段三：streaming=true 时头部显示「思考中」，正文增量追加
// ============================================================================
import { useEffect, useState } from 'react';
import styles from './ThinkingBlock.module.scss';

interface ThinkingBlockProps {
  /** 进度步骤文字（后端 progress 分块） */
  steps: string[];
  /** 完整思考链文本（历史消息 metadata.reasoning） */
  reasoning?: string;
  /** 流式累积中的思考链文本（thinking 分块实时追加） */
  streamingText?: string;
  /** 是否仍在思考中 */
  streaming?: boolean;
}

const ThinkingBlock: React.FC<ThinkingBlockProps> = ({
  steps, reasoning, streamingText, streaming,
}) => {
  const [open, setOpen] = useState(false);
  const hasBody = (steps && steps.length > 0) || !!reasoning || !!streamingText;

  // 思考中自动展开，结束后由用户决定收放
  useEffect(() => {
    if (streaming) setOpen(true);
  }, [streaming]);

  if (!hasBody && !streaming) return null;

  return (
    <div className={styles.thinking}>
      <button type='button' className={styles.header} onClick={() => setOpen(!open)}>
        <svg
          className={[styles.chevron, open ? styles.chevronOpen : ''].join(' ')}
          width='12' height='12' viewBox='0 0 24 24' fill='none' stroke='currentColor'
          strokeWidth='2' strokeLinecap='round' strokeLinejoin='round'
        >
          <polyline points='9 18 15 12 9 6' />
        </svg>
        {streaming ? (
          <span className={styles.headerText}><span className={styles.pulseDot} />思考中…</span>
        ) : (
          <span className={styles.headerText}>已深度思考</span>
        )}
      </button>
      {open && (
        <div className={styles.body}>
          {steps && steps.length > 0 && (
            <ul className={styles.stepList}>
              {steps.map((text, i) => (
                <li key={i} className={styles.step}>
                  <span className={styles.stepMarker} />
                  <span>{text}</span>
                </li>
              ))}
            </ul>
          )}
          {(reasoning || streamingText) && (
            <p className={styles.reasoning}>
              {reasoning || ''}
              {streamingText || ''}
              {streaming && <span className={styles.cursor} />}
            </p>
          )}
        </div>
      )}
    </div>
  );
};

export default ThinkingBlock;
