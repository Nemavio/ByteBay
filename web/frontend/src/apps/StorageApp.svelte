<script>
  import { onMount } from 'svelte'
  import { api } from '../lib/api.js'

  let disks = $state([])
  let loading = $state(true)
  let error = $state('')

  onMount(() => { load() })

  async function load() {
    loading = true
    error = ''
    try {
      disks = await api.disks()
    } catch (e) {
      error = e.message
    } finally {
      loading = false
    }
  }

  function fmt(bytes) {
    if (!bytes) return '—'
    const u = ['o', 'Ko', 'Mo', 'Go', 'To']
    let i = 0
    let n = bytes
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
    return `${n.toFixed(1)} ${u[i]}`
  }
</script>

{#if loading}
  <p>Chargement…</p>
{:else if error}
  <p class="err">{error}</p>
{:else}
  <table>
    <thead>
      <tr><th>Disque</th><th>Taille</th><th>Modèle</th><th>Montage</th><th>RAID</th></tr>
    </thead>
    <tbody>
      {#each disks as d}
        <tr>
          <td><code>{d.path}</code></td>
          <td>{fmt(d.size_bytes)}</td>
          <td>{d.model || '—'}</td>
          <td>{d.mountpoint || '—'}</td>
          <td>
            {#if d.in_raid}
              <code>{d.raid_member}</code>
            {:else}
              —
            {/if}
          </td>
        </tr>
      {/each}
    </tbody>
  </table>
{/if}

<style>
  .err { color: var(--bb-danger); }
  code { font-size: 12px; }
</style>
