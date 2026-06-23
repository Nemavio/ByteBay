<script>
  /** @type {{ health: { web?: boolean, agent?: boolean, engine?: boolean } | null, compact?: boolean }} */
  let { health, compact = false } = $props()

  const items = [
    { key: 'web', label: 'Panel web', role: 'Interface de gestion' },
    { key: 'agent', label: 'Agent hôte', role: 'RAID, SMART, montages' },
    { key: 'engine', label: 'Engine', role: 'NFS, Samba, FTP' },
  ]
</script>

<div class="cards" class:compact>
  {#each items as item}
    {@const ok = health?.[item.key]}
    <div class="card">
      <span class="lbl">{item.label}</span>
      {#if health}
        <span class="badge" class:ok={ok} class:warn={!ok}>
          {ok ? 'En ligne' : 'Hors ligne'}
        </span>
      {:else}
        <span class="badge">…</span>
      {/if}
      {#if !compact}
        <span class="role">{item.role}</span>
      {/if}
    </div>
  {/each}
</div>

<style>
  .cards {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 10px;
  }
  .cards.compact {
    gap: 12px;
  }
  .card {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.03);
  }
  .compact .card {
    min-width: 120px;
    background: var(--bb-panel);
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    padding: 14px;
    gap: 8px;
  }
  .lbl {
    font-size: 12px;
    font-weight: 600;
  }
  .compact .lbl {
    font-size: 11px;
    font-weight: 500;
    color: var(--bb-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .role {
    font-size: 10px;
    color: var(--bb-muted);
    line-height: 1.35;
  }
  .badge {
    display: inline-block;
    width: fit-content;
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 10px;
    font-weight: 600;
    background: rgba(255, 255, 255, 0.06);
    color: var(--bb-muted);
  }
  .badge.ok {
    background: rgba(62, 207, 142, 0.2);
    color: var(--bb-ok);
  }
  .badge.warn {
    background: rgba(231, 76, 92, 0.15);
    color: var(--bb-danger);
  }
  @media (max-width: 640px) {
    .cards {
      grid-template-columns: 1fr;
    }
  }
</style>
