<script>
  import Login from './Login.svelte'
  import Window from './lib/Window.svelte'
  import DashboardApp from './apps/DashboardApp.svelte'
  import StorageApp from './apps/StorageApp.svelte'
  import SmartApp from './apps/SmartApp.svelte'
  import RaidApp from './apps/RaidApp.svelte'
  import MountsApp from './apps/MountsApp.svelte'
  import SharesApp from './apps/SharesApp.svelte'
  import UsersApp from './apps/UsersApp.svelte'
  import AccessApp from './apps/AccessApp.svelte'
  import NetworkApp from './apps/NetworkApp.svelte'
  import FilesApp from './apps/FilesApp.svelte'
  import LogsApp from './apps/LogsApp.svelte'
  import { api, clearToken, getToken } from './lib/api.js'
  import { uid } from './lib/uid.js'
  import { setDesktop } from './lib/desktop.js'
  import { nextWindowPosition, childWindowPosition, desktopIconPositions } from './lib/windowPlacement.js'
  import WindowHost from './lib/WindowHost.svelte'
  import HealthOverlay from './lib/HealthOverlay.svelte'
  import HousekeepingNotifications from './lib/HousekeepingNotifications.svelte'
  import MessageModal from './lib/MessageModal.svelte'
  import ContextMenu from './lib/ContextMenu.svelte'
  import { setContextMenu, openContextMenu } from './lib/contextMenu.js'
  import { poll, POLL_LIST_MS } from './lib/poll.js'
  import { fetchHealthWithRetry } from './lib/health.js'
  import { dismissTopOverlay } from './lib/escape.js'

  const APPS = [
    { id: 'dashboard', title: 'Tableau de bord', icon: '🏠', component: DashboardApp, w: 760, h: 580 },
    { id: 'files', title: 'Explorateur', icon: '📂', component: FilesApp, w: 900, h: 560 },
    { id: 'storage', title: 'Stockage', icon: '💾', component: StorageApp, w: 720, h: 400 },
    { id: 'smart', title: 'SMART', icon: '🩺', component: SmartApp, w: 680, h: 420 },
    { id: 'raid', title: 'RAID', icon: '🔗', component: RaidApp, w: 760, h: 560 },
    { id: 'mounts', title: 'Montages', icon: '📀', component: MountsApp, w: 760, h: 480 },
    { id: 'shares', title: 'Partages', icon: '📁', component: SharesApp, w: 760, h: 620 },
    { id: 'users', title: 'Utilisateurs', icon: '👤', component: UsersApp, w: 720, h: 420 },
    { id: 'access', title: "Droits d'accès", icon: '🔐', component: AccessApp, w: 720, h: 580 },
    { id: 'network', title: 'Paramètres réseau', icon: '🌐', component: NetworkApp, w: 820, h: 560 },
    { id: 'logs', title: 'Journaux', icon: '📋', component: LogsApp, w: 900, h: 560 },
  ]

  const APP_BY_ID = Object.fromEntries(APPS.map((a) => [a.id, a]))

  /** @type {Map<string, { component: any, props: object }>} */
  const customMeta = new Map()

  let authed = $state(!!getToken())
  let user = $state(null)
  let windows = $state([])
  let zTop = $state(10)
  let clock = $state('')
  let menuOpen = $state(false)
  let openedDefault = false
  let health = $state(null)
  let healthChecking = $state(false)
  let healthFailures = $state(0)
  let iconLayout = $state(desktopIconPositions(APPS.length))
  let housekeepingItems = $state([])
  let recoveringRaid = $state('')
  let notice = $state({ open: false, title: '', message: '', variant: 'info' })
  let ctxMenu = $state({ open: false, x: 0, y: 0, items: [] })
  let flashKey = $state(null)
  /** @type {ReturnType<typeof setTimeout> | null} */
  let flashTimer = null

  function showContextMenu(x, y, items) {
    ctxMenu = { open: true, x, y, items }
  }

  function closeContextMenu() {
    ctxMenu = { ...ctxMenu, open: false, items: [] }
  }

  const ctxMenuApi = { show: showContextMenu, close: closeContextMenu }

  let servicesDown = $derived(
    health !== null && (!health.agent || !health.engine) && healthFailures >= 2
  )

  async function refreshHousekeeping() {
    if (servicesDown) return
    try {
      const report = await api.housekeeping()
      housekeepingItems = report.items || []
    } catch {
      housekeepingItems = []
    }
  }

  async function recoverOrphanRaid(uuid) {
    recoveringRaid = uuid
    try {
      await api.recoverRaid(uuid)
      await refreshHousekeeping()
      openAppById('raid')
      notice = { open: true, title: 'RAID démarré', message: 'Le volume RAID a été démarré avec succès.', variant: 'success' }
    } catch (e) {
      notice = { open: true, title: 'Échec du démarrage RAID', message: e.message, variant: 'error' }
    } finally {
      recoveringRaid = ''
    }
  }

  function refreshIconLayout() {
    iconLayout = desktopIconPositions(APPS.length)
  }

  function bumpZ() {
    return windows.length ? Math.max(...windows.map((w) => w.z)) + 1 : 11
  }

  function openCustomWindow({ title, component, props = {}, w = 560, h = 420, from }) {
    menuOpen = false
    const key = uid()
    customMeta.set(key, { component, props })
    const pos = from
      ? childWindowPosition(from, w, h, windows)
      : nextWindowPosition(windows.length, w, h)
    const z = bumpZ()
    zTop = z
    windows = [
      ...windows,
      { key, id: `custom-${key}`, title, custom: true, x: pos.x, y: pos.y, w, h, z },
    ]
  }

  function openApp(app, offset = 0) {
    menuOpen = false
    const existing = windows.find((w) => w.id === app.id && !w.custom)
    if (existing) {
      focusWindow(existing.key, { pulse: true })
      return
    }
    const key = uid()
    const pos = nextWindowPosition(windows.length + offset, app.w, app.h)
    const z = bumpZ()
    zTop = z
    windows = [
      ...windows,
      {
        key,
        id: app.id,
        title: app.title,
        custom: false,
        x: pos.x,
        y: pos.y,
        w: app.w,
        h: app.h,
        z,
      },
    ]
  }

  function openAppById(id) {
    const app = APP_BY_ID[id]
    if (app) openApp(app)
  }

  setDesktop({ openCustomWindow, openAppById, closeWindow })
  setContextMenu(ctxMenuApi)

  function onDesktopContextMenu(e) {
    if (e.target.closest('.window, .notifications, .ctx-menu')) return
    const iconBtn = e.target.closest('.icons .icon')
    if (iconBtn) {
      const idx = Number(iconBtn.dataset.idx)
      const app = APPS[idx]
      if (app) {
        openContextMenu(e, [
          { label: `Ouvrir ${app.title}`, icon: app.icon, action: () => openApp(app, idx) },
        ], ctxMenuApi)
      }
      return
    }
    openContextMenu(e, [
      { label: 'Explorateur de fichiers', icon: '📂', action: () => openAppById('files') },
      { label: 'Tableau de bord', icon: '🏠', action: () => openAppById('dashboard') },
      { label: 'Journaux', icon: '📋', action: () => openAppById('logs') },
      { separator: true },
      {
        label: 'Actualiser',
        icon: '🔄',
        action: () => {
          refreshHealth()
          refreshHousekeeping()
        },
      },
    ], ctxMenuApi)
  }

  async function refreshHealth() {
    healthChecking = true
    try {
      const next = await fetchHealthWithRetry(() => api.health())
      if (next) {
        health = next
        healthFailures = 0
      } else {
        healthFailures++
      }
    } finally {
      healthChecking = false
    }
  }

  $effect(() => {
    if (authed) {
      api.me().then((u) => (user = u)).catch(() => logout())
      if (!openedDefault) {
        openedDefault = true
        openApp(APPS[0])
      }
      refreshHealth()
      refreshIconLayout()
      refreshHousekeeping()
      const healthId = setInterval(refreshHealth, 5000)
      const housekeepingStop = poll(refreshHousekeeping, POLL_LIST_MS)
      const tick = () => {
        clock = new Date().toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
      }
      tick()
      const clockId = setInterval(tick, 30_000)
      return () => {
        clearInterval(healthId)
        clearInterval(clockId)
        housekeepingStop()
        housekeepingItems = []
      }
    }
    openedDefault = false
    const tick = () => {
      clock = new Date().toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    }
    tick()
    const clockId = setInterval(tick, 30_000)
    return () => clearInterval(clockId)
  })

  function onLogin() {
    authed = true
  }

  function logout() {
    clearToken()
    authed = false
    user = null
    windows = []
    customMeta.clear()
    menuOpen = false
  }

  function closeWindow(key) {
    if (flashKey === key) flashKey = null
    windows = windows.filter((w) => w.key !== key)
    customMeta.delete(key)
  }

  function closeTopWindow() {
    if (!windows.length) return
    const top = windows.reduce((a, b) => (a.z > b.z ? a : b))
    closeWindow(top.key)
  }

  function patchWindow(key, patch) {
    windows = windows.map((w) => (w.key === key ? { ...w, ...patch } : w))
  }

  function focusWindow(key, { pulse = false } = {}) {
    const z = bumpZ()
    zTop = z
    windows = windows.map((w) => (w.key === key ? { ...w, z } : w))
    if (pulse) {
      flashKey = key
      if (flashTimer) clearTimeout(flashTimer)
      flashTimer = setTimeout(() => {
        flashKey = null
        flashTimer = null
      }, 600)
    }
  }

  function onKeydown(e) {
    if (e.key !== 'Escape' || servicesDown) return

    if (ctxMenu.open) {
      e.preventDefault()
      e.stopImmediatePropagation()
      closeContextMenu()
      return
    }
    if (notice.open) {
      e.preventDefault()
      e.stopImmediatePropagation()
      notice = { ...notice, open: false }
      return
    }
    if (menuOpen) {
      e.preventDefault()
      e.stopImmediatePropagation()
      menuOpen = false
      return
    }
    if (dismissTopOverlay()) {
      e.preventDefault()
      e.stopImmediatePropagation()
      return
    }

    e.preventDefault()
    e.stopImmediatePropagation()
    closeTopWindow()
  }

  function toggleMenu() {
    menuOpen = !menuOpen
  }
</script>

<svelte:window
  onclick={() => (menuOpen = false)}
  onkeydown={onKeydown}
  onresize={refreshIconLayout}
/>

{#if !authed}
  <Login onLogin={onLogin} />
{:else}
  <div class="desktop">
    <header class="taskbar">
      <div class="start-wrap">
        <button class="start" onclick={(e) => { e.stopPropagation(); toggleMenu() }}>⊞ ByteBay</button>
        {#if menuOpen}
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <!-- svelte-ignore a11y_no_static_element_interactions -->
          <div class="start-menu" onclick={(e) => e.stopPropagation()}>
            <p class="menu-title">Applications</p>
            {#each APPS as app}
              <button class="menu-item" onclick={() => openApp(app)}>
                <span>{app.icon}</span> {app.title}
              </button>
            {/each}
          </div>
        {/if}
      </div>
      <div class="task-apps">
        {#each windows as w}
          <button class="task" class:active={w.z === Math.max(...windows.map((x) => x.z))} onclick={() => focusWindow(w.key, { pulse: true })}>
            {w.title}
          </button>
        {/each}
      </div>
      <div class="tray">
        <span class="user">{user?.username ?? ''}</span>
        <button class="ghost small" onclick={logout}>Déconnexion</button>
        <span class="clock">{clock}</span>
      </div>
    </header>

    <div class="workspace" oncontextmenu={onDesktopContextMenu}>
      <div class="wallpaper"></div>
      <HousekeepingNotifications
        items={housekeepingItems}
        recovering={recoveringRaid}
        onrecover={recoverOrphanRaid}
        onopenapp={openAppById}
      />
      <div class="icons">
        {#each APPS as app, i}
          {@const pos = iconLayout.positions[i]}
          <button
            class="icon"
            data-idx={i}
            style:left="{pos.x}px"
            style:top="{pos.y}px"
            onclick={() => openApp(app, i)}
            title={app.title}
          >
            <span class="emoji">{app.icon}</span>
            <span class="label">{app.title}</span>
          </button>
        {/each}
      </div>

      {#each windows as w (w.key)}
        {#if w.custom}
          {@const meta = customMeta.get(w.key)}
          {#if meta}
            <Window
              title={w.title}
              x={w.x}
              y={w.y}
              width={w.w}
              height={w.h}
              z={w.z}
              flash={flashKey === w.key}
              onclose={() => closeWindow(w.key)}
              onfocus={() => focusWindow(w.key)}
              onmove={({ x, y }) => patchWindow(w.key, { x, y })}
              onresize={({ x, y, w: nw, h: nh }) => patchWindow(w.key, { x, y, w: nw, h: nh })}
            >
              <WindowHost x={w.x} y={w.y} w={w.w} h={w.h} winKey={w.key}>
                <svelte:component this={meta.component} {...meta.props} />
              </WindowHost>
            </Window>
          {/if}
        {:else if APP_BY_ID[w.id]}
          <Window
            title={w.title}
            x={w.x}
            y={w.y}
            width={w.w}
            height={w.h}
            z={w.z}
            flash={flashKey === w.key}
            onclose={() => closeWindow(w.key)}
            onfocus={() => focusWindow(w.key)}
            onmove={({ x, y }) => patchWindow(w.key, { x, y })}
            onresize={({ x, y, w: nw, h: nh }) => patchWindow(w.key, { x, y, w: nw, h: nh })}
          >
            <WindowHost x={w.x} y={w.y} w={w.w} h={w.h} winKey={w.key}>
              <svelte:component this={APP_BY_ID[w.id].component} />
            </WindowHost>
          </Window>
        {/if}
      {/each}
    </div>

    {#if servicesDown}
      <HealthOverlay
        {health}
        checking={healthChecking}
        onretry={refreshHealth}
        onlogout={logout}
      />
    {/if}

    <MessageModal
      open={notice.open}
      title={notice.title}
      message={notice.message}
      variant={notice.variant}
      onclose={() => (notice = { ...notice, open: false })}
    />

    <ContextMenu
      open={ctxMenu.open}
      x={ctxMenu.x}
      y={ctxMenu.y}
      items={ctxMenu.items}
      onclose={closeContextMenu}
    />
  </div>
{/if}

<style>
  .desktop { display: flex; flex-direction: column; height: 100%; min-height: 0; overflow: hidden; }
  .taskbar {
    flex-shrink: 0; height: 44px; background: var(--bb-taskbar);
    border-bottom: 1px solid var(--bb-border); display: flex; align-items: center;
    padding: 0 8px; gap: 8px; z-index: 100;
  }
  .start-wrap { position: relative; }
  .start { font-weight: 600; padding: 6px 14px; font-size: 13px; }
  .start-menu {
    position: absolute; top: calc(100% + 4px); left: 0; min-width: 220px;
    background: var(--bb-panel); border: 1px solid var(--bb-border);
    border-radius: 8px; box-shadow: var(--bb-shadow); padding: 8px; z-index: 200;
  }
  .menu-title { font-size: 11px; color: var(--bb-muted); text-transform: uppercase; padding: 4px 8px 8px; }
  .menu-item {
    display: flex; align-items: center; gap: 10px; width: 100%; text-align: left;
    padding: 8px 10px; background: transparent; color: var(--bb-text); font-size: 13px; border-radius: 6px;
  }
  .menu-item:hover { background: rgba(61, 155, 233, 0.2); }
  .workspace { position: relative; flex: 1; min-height: 0; overflow: hidden; }
  .wallpaper {
    position: absolute; inset: 0;
    background: linear-gradient(160deg, #0c1f38 0%, #0a1628 40%, #061018 100%),
      url("data:image/svg+xml,%3Csvg width='60' height='60' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M0 30h60M30 0v60' stroke='%231a3050' stroke-width='0.5' opacity='0.3'/%3E%3C/svg%3E");
  }
  .icons {
    position: absolute;
    inset: 0;
    z-index: 1;
    pointer-events: none;
    overflow: hidden;
  }
  .icon {
    position: absolute;
    display: flex; flex-direction: column; align-items: center; gap: 4px; width: 76px;
    padding: 8px 4px; background: transparent; color: var(--bb-text); border-radius: 8px;
    pointer-events: auto;
  }
  .icon:hover { background: rgba(255,255,255,0.08); }
  .emoji { font-size: 28px; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.4)); }
  .label {
    font-size: 11px; text-align: center; text-shadow: 0 1px 3px rgba(0,0,0,0.8); line-height: 1.2;
    display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden;
    max-width: 76px;
  }
  .task-apps { display: flex; gap: 4px; flex: 1; overflow: hidden; }
  .task {
    background: rgba(61, 155, 233, 0.15); border: 1px solid transparent; padding: 4px 12px;
    font-size: 12px; max-width: 160px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--bb-text);
  }
  .task:hover, .task.active { background: rgba(61, 155, 233, 0.3); border-color: var(--bb-border); }
  .tray { display: flex; align-items: center; gap: 10px; font-size: 12px; color: var(--bb-muted); }
  .small { padding: 4px 10px; font-size: 11px; }
  .clock { min-width: 44px; text-align: right; color: var(--bb-text); }
</style>
