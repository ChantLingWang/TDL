// ============================================================================
// 消息列表 —— 用户消息 + AI step 流
// ============================================================================
import { useEffect, useRef } from 'react';
import type { StepStream, WsMessage } from '../types/chat';
import MessageBubble from './MessageBubble';
import StepTimeline from './StepTimeline';
import styles from './MessageList.module.scss';

interface MessageListProps {
  messages: WsMessage[];
  userId: string;
  conversationId: string;
  streams: Record<string, StepStream>;
  hasMoreHistory: boolean;
  loadingHistory: boolean;
  onLoadMore: () => void;
}

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

interface ResultView {
  content: string;
  status: string;
  reason: string;
  time?: number;
}

const resultText = (stream: StepStream): ResultView => {
  for (let i = stream.steps.length - 1; i >= 0; i -= 1) {
    const step = stream.steps[i];
    if (step.kind === 'RESULT') {
      return {
        content: step.payload.result || '',
        status: step.payload.status || '',
        reason: step.payload.reason || '',
        time: step.time,
      };
    }
  }
  return { content: '', status: '', reason: '' };
};

const MessageList: React.FC<MessageListProps> = ({
  messages, userId, conversationId, streams, hasMoreHistory, loadingHistory, onLoadMore,
}) => {
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, streams]);

  const grouped = groupMessages(messages);
  const streamEntries = Object.entries(streams)
    .filter(([, s]) => s.conversation_id === conversationId)
    .sort((a, b) => (a[1].steps[0]?.time || 0) - (b[1].steps[0]?.time || 0));

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
        const mine = group[0]?.sender === userId;
        return (
          <div key={gi}>
            {group.map((m, mi) => (
              <MessageBubble
                key={m.message_id || (gi + '-' + mi)}
                message={m}
                mine={mine}
                showName={mi === 0}
                showTime={mi === group.length - 1}
              />
            ))}
          </div>
        );
      })}

      {streamEntries.map(([id, stream]) => {
        const { content, status, reason, time } = resultText(stream);
        let statusText = '';
        if (status === 'canceled') {
          statusText = '已取消' + (reason ? '：' + reason : '');
        } else if (status === 'failed') {
          statusText = '失败' + (reason ? '：' + reason : '');
        } else if (stream.done && !content) {
          statusText = '（空回复）';
        }
        const finalContent = content || statusText;

        return (
          <div key={'stream-' + id} className={styles.streamBlock}>
            <StepTimeline steps={stream.steps} streaming={!stream.done} />
            {stream.done && finalContent && (
              <MessageBubble
                message={{
                  type: 'chat',
                  sender: stream.sender || 'ai-assistant',
                  content: finalContent,
                  time,
                  message_id: id,
                  group_id: stream.conversation_id,
                  reply_to_msg_id: stream.reply_to_msg_id,
                }}
                mine={false}
                showName
                showTime={false}
              />
            )}
          </div>
        );
      })}

      <div ref={endRef} />
    </div>
  );
};

export default MessageList;
