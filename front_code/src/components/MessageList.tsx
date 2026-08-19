// ============================================================================
// 消息列表 —— 按发送者/时间分组，挂靠思考步骤，滚动到底
// ============================================================================
import { useEffect, useRef } from 'react';
import type { WsMessage } from '../types/chat';
import MessageBubble from './MessageBubble';
import styles from './MessageList.module.scss';

/** 一条正在流式累积的 AI 回复 */
export interface StreamEntry {
  group_id: string;
  sender: string;
  reasoning: string;
  content: string;
  steps: string[];
  done: boolean;
  time?: number;
  reply_to_msg_id?: string;
}

interface MessageListProps {
  messages: WsMessage[];
  userId: string;
  /** 按用户消息 message_id 键控的思考步骤 */
  progressSteps: Record<string, string[]>;
  /** 当前会话正在流式累积的回复（key 为回复 message_id） */
  streams: Record<string, StreamEntry>;
  hasMoreHistory: boolean;
  loadingHistory: boolean;
  onLoadMore: () => void;
}

/** 同一发送者 3 分钟内的消息并为一组 */
function groupMessages(messages: WsMessage[]): WsMessage[][] {
  return messages.reduce<WsMessage[][]>((acc, msg) => {
    const prev = acc[acc.length - 1];
    if (prev) {
      const last = prev[prev.length - 1];
      if (last.sender === msg.sender && last.time && msg.time
        && (msg.time - last.time) < 180_000) {
        prev.push(msg);
        return acc;
      }
    }
    acc.push([msg]);
    return acc;
  }, []);
}

const MessageList: React.FC<MessageListProps> = ({
  messages, userId, progressSteps, streams, hasMoreHistory, loadingHistory, onLoadMore,
}) => {
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, progressSteps, streams]);

  const grouped = groupMessages(messages);

  // 流式条目：跳过已有最终消息的（以服务端整块为准）
  const finalIds = new Set(messages.filter((m) => !m.kind).map((m) => m.message_id));
  const streamEntries = Object.entries(streams)
    .filter(([id]) => !finalIds.has(id))
    .sort((a, b) => (a[1].time || 0) - (b[1].time || 0));

  return (
    <div className={styles.list}>
      {hasMoreHistory && (
        <div className={styles.loadMoreRow}>
          <button className={styles.loadMoreBtn} onClick={onLoadMore} disabled={loadingHistory}>
            {loadingHistory ? '加载中…' : '加载更早消息'}
          </button>
        </div>
      )}
      {grouped.map((group, gi) => {
        const last = group[group.length - 1];
        const mine = group[0]?.sender === userId;
        const steps = mine ? progressSteps[last.message_id || ''] : undefined;
        return (
          <div key={gi}>
            {group.map((m, mi) => (
              <MessageBubble
                key={m.message_id || (gi + '-' + mi)}
                message={m}
                mine={mine}
                showName={mi === 0}
                showTime={mi === group.length - 1}
                thinkingSteps={steps}
              />
            ))}
          </div>
        );
      })}
      {streamEntries.map(([id, s]) => (
        <MessageBubble
          key={'stream-' + id}
          message={{
            type: 'chat',
            sender: s.sender,
            content: s.content,
            time: s.time,
            message_id: id,
            group_id: s.group_id,
            reply_to_msg_id: s.reply_to_msg_id,
          }}
          mine={false}
          showName={true}
          showTime={false}
          thinkingSteps={s.steps}
          streamingText={s.reasoning}
          streamingContent={s.content}
          streaming={!s.done}
        />
      ))}
      <div ref={endRef} />
    </div>
  );
};

export default MessageList;
