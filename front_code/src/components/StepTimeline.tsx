// ============================================================================
// Step 时间线 —— 渲染 THINK / TOOL_CALL / TOOL_RESULT
// ============================================================================
import type { StepEntry, ToolCallItem, ToolResultItem } from '../types/chat';
import ThinkingBlock from './ThinkingBlock';
import styles from './StepTimeline.module.scss';

interface StepTimelineProps {
  steps: StepEntry[];
  streaming: boolean;
}

interface ToolView extends ToolCallItem {
  payload?: unknown;
  status?: string;
  duration_ms?: number;
  error?: string;
}

const StepTimeline: React.FC<StepTimelineProps> = ({ steps, streaming }) => {
  const think = steps
    .filter((s) => s.kind === 'THINK')
    .map((s) => s.payload.think || '')
    .filter(Boolean)
    .join('\n\n');

  const calls = new Map<string, ToolCallItem>();
  const results = new Map<string, ToolResultItem>();
  steps.forEach((s) => {
    (s.payload.calls || []).forEach((call) => calls.set(call.id, call));
    (s.payload.results || []).forEach((result) => results.set(result.id, result));
  });

  const tools: ToolView[] = [];
  calls.forEach((call, id) => {
    tools.push({ ...call, ...(results.get(id) || {}) });
  });
  results.forEach((result, id) => {
    if (!calls.has(id)) tools.push({ ...result });
  });

  const hasResult = steps.some((s) => s.kind === 'RESULT');

  if (!think && tools.length === 0 && !streaming) return null;

  return (
    <div className={styles.timeline}>
      {(think || streaming) && (
        <ThinkingBlock
          steps={[]}
          reasoning={think}
          streaming={streaming && !hasResult}
        />
      )}

      {tools.map((tool) => (
        <div key={tool.id} className={styles.tool}>
          <div className={styles.head}>
            <span className={styles.name}>{tool.name || '工具'}</span>
            <span className={styles.status} data-status={tool.status || 'requested'}>
              {tool.status || '调用中'}
              {typeof tool.duration_ms === 'number' ? ' · ' + tool.duration_ms + 'ms' : ''}
            </span>
          </div>
          {tool.input !== undefined && (
            <pre className={styles.code}>{JSON.stringify(tool.input, null, 2)}</pre>
          )}
          {tool.payload !== undefined && (
            <pre className={styles.code}>
              {typeof tool.payload === 'string'
                ? tool.payload
                : JSON.stringify(tool.payload, null, 2)}
            </pre>
          )}
          {tool.error && <div className={styles.error}>{tool.error}</div>}
        </div>
      ))}

      {streaming && <div className={styles.pending}>处理中…</div>}
    </div>
  );
};

export default StepTimeline;
