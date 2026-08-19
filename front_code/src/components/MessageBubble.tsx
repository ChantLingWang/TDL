// ============================================================================
// 消息气泡 —— 单条消息的渲染单元
// 阶段一：纯文本内容；阶段三：接入 Markdown 与打字机流式
// ============================================================================
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { WsMessage } from '../types/chat';
import ThinkingBlock from './ThinkingBlock';
import styles from './MessageBubble.module.scss';

interface MessageBubbleProps {
  message: WsMessage;
  mine: boolean;
  /** 是否显示发送者名（同组第一条） */
  showName: boolean;
  /** 是否显示时间（同组最后一条） */
  showTime: boolean;
  /** 挂在该消息下的思考步骤 */
  thinkingSteps?: string[];
  /** 流式累积中的正文（覆盖 message.content 显示，打字机效果） */
  streamingContent?: string;
  /** 流式累积中的思考文本 */
  streamingText?: string;
  /** 是否流式中（隐藏时间、显示光标） */
  streaming?: boolean;
}

const initials = (name: string): string => (name || '?').slice(0, 2).toUpperCase();

const fmtTime = (ts?: number): string => {
  if (!ts) return '';
  const d = new Date(ts);
  const h = String(d.getHours()).padStart(2, '0');
  const m = String(d.getMinutes()).padStart(2, '0');
  return h + ':' + m;
};

const MessageBubble: React.FC<MessageBubbleProps> = ({
  message, mine, showName, showTime, thinkingSteps,
  streamingContent, streamingText, streaming,
}) => {
  const sender = message.sender || '?';
  const hasThinking = (thinkingSteps && thinkingSteps.length > 0)
    || !!message.reasoning || !!streamingText || !!streaming;
  const content = streaming ? (streamingContent || '') : message.content;

  return (
    <div className={[styles.row, mine ? styles.rowMine : ''].join(' ')}>
      {!mine && (
        <div className={styles.avatar}>{initials(sender)}</div>
      )}
      <div className={[styles.stack, mine ? styles.stackMine : ''].join(' ')}>
        {!mine && showName && <div className={styles.name}>{sender}</div>}
        {hasThinking && (
          <ThinkingBlock
            steps={thinkingSteps || []}
            reasoning={message.reasoning}
            streamingText={streamingText}
            streaming={streaming}
          />
        )}
        {content && (
          <div className={[styles.bubble, mine ? styles.bubbleMine : ''].join(' ')}>
            {streaming ? (
              <div className={styles.text}>
                {content}
                <span className={styles.cursor} />
              </div>
            ) : mine ? (
              <div className={styles.text}>{content}</div>
            ) : (
              <div className={[styles.text, styles.md].join(' ')}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
              </div>
            )}
          </div>
        )}
        {!streaming && showTime && message.time && (
          <div className={styles.time}>{fmtTime(message.time)}</div>
        )}
      </div>
    </div>
  );
};

export default MessageBubble;
