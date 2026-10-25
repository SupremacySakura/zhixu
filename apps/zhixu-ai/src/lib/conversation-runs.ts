type Conversation = { id: string; resource: string };
type Run = { controller: AbortController; settled: Promise<void>; finish: () => void };

// Process-local barrier only. Go's durable tombstone must reject new turns after
// deletion across restarts/replicas; this cannot cancel a run on another AI replica.
export class ConversationRuns {
  private active = new Map<string, Set<Run>>();
  private deleted = new Set<string>();
  private key(c: Conversation) { return JSON.stringify([c.resource, c.id]); }

  register(c: Conversation | undefined, controller: AbortController): (() => void) | null {
    if (!c) return () => {};
    const key = this.key(c);
    if (this.deleted.has(key)) return null;
    let finish!: () => void;
    const settled = new Promise<void>((resolve) => { finish = resolve; });
    const run = { controller, settled, finish };
    const runs = this.active.get(key) ?? new Set<Run>();
    runs.add(run);
    this.active.set(key, runs);
    return () => {
      runs.delete(run);
      if (!runs.size) this.active.delete(key);
      finish();
    };
  }

  async delete(c: Conversation, remove: () => Promise<void>): Promise<void> {
    const key = this.key(c);
    // Fence before any await: no new local streams can start during deletion.
    this.deleted.add(key);
    const runs = [...(this.active.get(key) ?? [])];
    for (const run of runs) run.controller.abort();
    await Promise.all(runs.map((run) => run.settled));
    // Delete after SDK stream/onFinish has settled, not merely after abort().
    await remove();
  }
}

export const conversationRuns = new ConversationRuns();
