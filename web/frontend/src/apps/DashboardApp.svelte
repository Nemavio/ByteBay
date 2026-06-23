<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'
  import HealthStatus from '../lib/HealthStatus.svelte'
  import { poll, POLL_LIST_MS } from '../lib/poll.js'

  let dash = $state(null)
  let error = $state('')

  async function refresh() {
    try {
      dash = await api.dashboard()
      error = ''
    } catch (e) {
      error = e.message
    }
  }

  onMount(() => {
    refresh()
    return poll(refresh, POLL_LIST_MS)
  })

  function fmt(bytes) {
    if (!bytes) return '—'
    const u = ['o', 'Ko', 'Mo', 'Go', 'To']
    let i = 0
    let n = bytes
    while (n >= 1024 && i < u.length - 1) {
      n /= 1024
      i++
    }
    return `${n.toFixed(1)} ${u[i]}`
  }

  function barClass(pct) {
    if (pct >= 90) return 'crit'
    if (pct >= 75) return 'warn'
    return 'ok'
  }

  function svcLabel(svc) {
    if (!svc) return { text: '—', cls: 'off' }
    if (svc.running && svc.enabled) return { text: 'Actif', cls: 'ok' }
    if (svc.running && !svc.enabled) return { text: 'En veille', cls: 'idle' }
    if (!svc.running && svc.enabled) return { text: 'Arrêté', cls: 'warn' }
    return { text: 'Inactif', cls: 'off' }
  }

  const host = $derived(dash?.host || {})
  const services = $derived(dash?.services || {})
  const platform = $derived(dash?.platform ?? null)
</script>

<h2>Bienvenue sur ByteBay</h2>
<p class="sub">Gestionnaire NAS léger et opensource</p>

{#if error}
  <p class="err">{error}</p>
{/if}

<section class="card">
  <h3>Services plateforme</h3>
  <HealthStatus health={platform} />
</section>

{#if host.cpu || host.memory}
  <section class="grid2">
    {#if host.cpu}
      <div class="card">
        <div class="card-head">
          <strong>CPU</strong>
          <span>{host.cpu.percent?.toFixed?.(0) ?? 0}% · {host.cpu.cores} cœurs</span>
        </div>
        <div class="bar-track">
          <div class="bar-fill {barClass(host.cpu.percent)}" style="width:{Math.min(100, host.cpu.percent || 0)}%"></div>
        </div>
        <p class="meta">Charge {host.cpu.load?.map((n) => n.toFixed(2)).join(' · ') || '—'}</p>
      </div>
    {/if}
    {#if host.memory}
      <div class="card">
        <div class="card-head">
          <strong>RAM</strong>
          <span>{fmt(host.memory.used_bytes)} / {fmt(host.memory.total_bytes)}</span>
        </div>
        <div class="bar-track">
          <div class="bar-fill {barClass(host.memory.percent)}" style="width:{Math.min(100, host.memory.percent || 0)}%"></div>
        </div>
        <p class="meta">{host.memory.percent?.toFixed?.(0) ?? 0}% utilisée</p>
      </div>
    {/if}
  </section>
{/if}

{#if services.samba || services.nfs || services.ftp}
  <section class="card">
    <h3>Services réseau</h3>
    <div class="svc-row">
      {#each [
        { key: 'Samba', svc: services.samba },
        { key: 'NFS', svc: services.nfs },
        { key: 'FTP', svc: services.ftp },
      ] as item}
        {@const st = svcLabel(item.svc)}
        <div class="svc">
          <span class="svc-name">{item.key}</span>
          <span class="badge {st.cls}">{st.text}</span>
          {#if item.svc?.shares}
            <span class="svc-meta">{item.svc.shares} partage{item.svc.shares > 1 ? 's' : ''}</span>
          {/if}
        </div>
      {/each}
    </div>
  </section>
{/if}

{#if host.interfaces?.length}
  <section class="card">
    <h3>Interfaces réseau</h3>
    <table class="compact">
      <thead>
        <tr><th>Interface</th><th>État</th><th>IPv4</th><th>IPv6</th></tr>
      </thead>
      <tbody>
        {#each host.interfaces as iface}
          <tr>
            <td><code>{iface.name}</code></td>
            <td><span class="state" class:up={iface.state === 'up'}>{iface.state || '—'}</span></td>
            <td class="addrs">{iface.ipv4?.length ? iface.ipv4.join(', ') : '—'}</td>
            <td class="addrs">{iface.ipv6?.length ? iface.ipv6.join(', ') : '—'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </section>
{/if}

{#if host.mounts}
  <section class="card">
    <h3>Montages</h3>
    {#if host.mounts.length === 0}
      <p class="muted">Aucun point de montage configuré.</p>
    {:else}
      <div class="mounts">
        {#each host.mounts as m}
          <div class="mount">
            <div class="mount-head">
              <strong>{m.name}</strong>
              <span>{m.mounted ? '✓ monté' : '— démonté'}</span>
            </div>
            <div class="bar-track">
              <div
                class="bar-fill {barClass(m.percent)}"
                style="width:{m.mounted && m.total_bytes ? Math.min(100, m.percent || 0) : 0}%"
              ></div>
            </div>
            <p class="meta">
              {#if m.mounted && m.total_bytes}
                {fmt(m.used_bytes)} / {fmt(m.total_bytes)} ({m.percent?.toFixed?.(0) ?? 0}%)
              {:else if !m.mounted}
                Non monté
              {:else}
                Espace indisponible
              {/if}
              · <code>{m.container_path}</code>
            </p>
          </div>
        {/each}
      </div>
    {/if}
  </section>
{/if}

<style>
  h2 { font-size: 20px; margin-bottom: 4px; }
  .sub { color: var(--bb-muted); margin-bottom: 16px; }
  h3 { font-size: 12px; color: var(--bb-muted); text-transform: uppercase; letter-spacing: 0.04em; margin: 0 0 10px; }
  .grid2 { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin: 14px 0; }
  .card {
    background: rgba(0, 0, 0, 0.15);
    border: 1px solid var(--bb-border);
    border-radius: 8px;
    padding: 12px;
    margin-bottom: 10px;
  }
  .card-head { display: flex; justify-content: space-between; align-items: baseline; gap: 8px; margin-bottom: 8px; font-size: 12px; }
  .card-head strong { font-size: 13px; }
  .bar-track {
    height: 8px;
    background: var(--bb-border);
    border-radius: 4px;
    overflow: hidden;
  }
  .bar-fill {
    height: 100%;
    border-radius: 4px;
    transition: width 0.4s ease;
  }
  .bar-fill.ok { background: linear-gradient(90deg, #3d9be9, #5cb3ff); }
  .bar-fill.warn { background: linear-gradient(90deg, #e0a030, #f0c674); }
  .bar-fill.crit { background: linear-gradient(90deg, #d94a4a, #e74c5c); }
  .meta { margin: 6px 0 0; font-size: 11px; color: var(--bb-muted); }
  .svc-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 10px; }
  .svc {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 8px;
    border-radius: 6px;
    background: rgba(255, 255, 255, 0.03);
  }
  .svc-name { font-size: 13px; font-weight: 600; }
  .svc-meta { font-size: 10px; color: var(--bb-muted); }
  .badge {
    display: inline-block;
    width: fit-content;
    padding: 2px 8px;
    border-radius: 999px;
    font-size: 10px;
    font-weight: 600;
  }
  .badge.ok { background: rgba(62, 207, 142, 0.2); color: #3ecf8e; }
  .badge.warn { background: rgba(240, 198, 116, 0.2); color: #f0c674; }
  .badge.idle { background: rgba(61, 155, 233, 0.15); color: #7eb8e8; }
  .badge.off { background: rgba(255, 255, 255, 0.06); color: var(--bb-muted); }
  .compact { width: 100%; font-size: 11px; }
  .compact th { color: var(--bb-muted); font-weight: 500; text-align: left; padding: 4px 6px 6px 0; }
  .compact td { padding: 5px 6px 5px 0; vertical-align: top; }
  .addrs { word-break: break-all; max-width: 200px; }
  .state { text-transform: lowercase; color: var(--bb-muted); }
  .state.up { color: var(--bb-ok); }
  code { font-size: 10px; }
  .mounts { display: flex; flex-direction: column; gap: 12px; }
  .mount-head { display: flex; justify-content: space-between; font-size: 12px; margin-bottom: 6px; }
  .muted { color: var(--bb-muted); font-size: 12px; }
  .err { color: var(--bb-danger); font-size: 12px; margin: 8px 0; }
  @media (max-width: 640px) {
    .grid2, .svc-row { grid-template-columns: 1fr; }
  }
</style>
