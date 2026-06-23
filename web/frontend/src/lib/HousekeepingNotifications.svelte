<script>
  /** @type {{ items: object[], recovering?: string, onrecover: (uuid: string) => void, onopenapp?: (id: string) => void }} */
  let { items = [], recovering = '', onrecover, onopenapp } = $props()

  let actionable = $derived((items || []).filter((i) =>
    i.severity === 'action' ||
    i.kind?.endsWith('_job') ||
    i.kind === 'mount_format'
  ))

  function itemKey(item) {
    return item.id || item.details?.uuid || `${item.kind}:${item.message}`
  }

  function iconFor(item) {
    if (item.kind === 'raid_orphan' || item.kind === 'raid_inactive' || item.kind === 'raid_job' || item.kind === 'raid_resync') return '🔗'
    if (item.kind === 'mount_job' || item.kind === 'mount_format') return '📀'
    return '⚙️'
  }

  function toneFor(item) {
    if (item.severity === 'action') return 'action'
    if (item.kind === 'mount_format') return 'warn'
    return 'info'
  }
</script>

{#if actionable.length}
  <div class="notifications" aria-live="polite">
    {#each actionable as item (itemKey(item))}
      {@const tone = toneFor(item)}
      <article class="toast" class:action={tone === 'action'} class:warn={tone === 'warn'} class:info={tone === 'info'}>
        <div class="icon" aria-hidden="true">{iconFor(item)}</div>
        <div class="body">
          <p class="msg">{item.message}</p>
          {#if item.progress != null && item.kind?.endsWith('_job')}
            <div class="progress">
              <div class="bar" style:width="{Math.min(100, Math.max(0, item.progress))}%"></div>
            </div>
            <span class="pct">{item.progress}%</span>
          {/if}
          <div class="actions">
            {#if (item.kind === 'raid_orphan' || item.kind === 'raid_inactive') && item.details?.uuid}
              <button
                class="primary"
                disabled={!!recovering}
                onclick={() => onrecover?.(item.details.uuid)}
              >
                {recovering === item.details.uuid ? 'Démarrage…' : 'Démarrer (dégradé si besoin)'}
              </button>
              <button class="ghost" onclick={() => onopenapp?.('raid')}>Ouvrir RAID</button>
            {:else if item.kind === 'raid_job'}
              <button class="ghost" onclick={() => onopenapp?.('raid')}>Voir RAID</button>
            {:else if item.kind === 'mount_job'}
              <button class="ghost" onclick={() => onopenapp?.('mounts')}>Voir Montages</button>
            {/if}
          </div>
        </div>
      </article>
    {/each}
  </div>
{/if}

<style>
  .notifications {
    position: absolute;
    right: 16px;
    bottom: 16px;
    z-index: 40;
    display: flex;
    flex-direction: column-reverse;
    gap: 10px;
    width: min(380px, calc(100% - 32px));
    pointer-events: none;
  }

  .toast {
    pointer-events: auto;
    display: flex;
    gap: 12px;
    padding: 12px 14px;
    border-radius: 10px;
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    box-shadow: var(--bb-shadow);
    animation: slide-in 0.28s ease-out;
  }

  .toast.action {
    border-left: 3px solid #f0c674;
    background: linear-gradient(135deg, rgba(240, 198, 116, 0.1) 0%, var(--bb-panel) 55%);
  }

  .toast.warn {
    border-left: 3px solid #ffb86c;
    background: linear-gradient(135deg, rgba(255, 184, 108, 0.08) 0%, var(--bb-panel) 55%);
  }

  .toast.info {
    border-left: 3px solid var(--bb-accent);
    background: linear-gradient(135deg, rgba(61, 155, 233, 0.1) 0%, var(--bb-panel) 55%);
  }

  .icon {
    flex-shrink: 0;
    width: 32px;
    height: 32px;
    display: grid;
    place-items: center;
    font-size: 18px;
    background: rgba(0, 0, 0, 0.2);
    border-radius: 8px;
  }

  .body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .msg {
    font-size: 12px;
    line-height: 1.45;
    color: var(--bb-text);
  }

  .progress {
    height: 4px;
    border-radius: 99px;
    background: rgba(0, 0, 0, 0.25);
    overflow: hidden;
  }

  .bar {
    height: 100%;
    border-radius: 99px;
    background: var(--bb-accent);
    transition: width 0.35s ease;
  }

  .pct {
    font-size: 10px;
    color: var(--bb-muted);
    margin-top: -4px;
  }

  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .actions button {
    padding: 4px 10px;
    font-size: 11px;
  }

  .actions .primary {
    background: var(--bb-accent);
  }

  .actions .ghost {
    padding: 4px 10px;
    font-size: 11px;
  }

  @keyframes slide-in {
    from {
      opacity: 0;
      transform: translateY(12px) scale(0.98);
    }
    to {
      opacity: 1;
      transform: translateY(0) scale(1);
    }
  }
</style>
