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
  import { api, clearToken, getToken } from './lib/api.js'
  import { uid } from './lib/uid.js'
  import { setDesktop } from './lib/desktop.js'

  const APPS = [
    { id: 'dashboard', title: 'Tableau de bord', icon: '🏠', component: DashboardApp, w: 480, h: 320 },
    { id: 'files', title: 'Explorateur', icon: '📂', component: FilesApp, w: 900, h: 560 },
    { id: 'storage', title: 'Stockage', icon: '💾', component: StorageApp, w: 720, h: 400 },
    { id: 'smart', title: 'SMART', icon: '🩺', component: SmartApp, w: 680, h: 420 },
    { id: 'raid', title: 'RAID', icon: '🔗', component: RaidApp, w: 760, h: 560 },
    { id: 'mounts', title: 'Montages', icon: '📀', component: MountsApp, w: 760, h: 480 },
    { id: 'shares', title: 'Partages', icon: '📁', component: SharesApp, w: 720, h: 420 },
    { id: 'users', title: 'Utilisateurs', icon: '👤', component: UsersApp, w: 720, h: 420 },
    { id: 'access', title: "Droits d'accès", icon: '🔐', component: AccessApp, w: 720, h: 480 },
    { id: 'network', title: 'Paramètres réseau', icon: '🌐', component: NetworkApp, w: 820, h: 560 },
  ]

  const APP_BY_ID = Object.fromEntries(APPS.map((a) => [a.id, a]))
  const WORKSPACE_TOP = 52

  /** @type {Map<string, { component: any, props: object }>} */
  const customMeta = new Map()

  let authed = $state(!!getToken())
  let user = $state(null)
  let windows = $state([])
  let zTop = $state(10)
  let clock = $state('')
  let menuOpen = $state(false)
  let openedDefault = false

  function openCustomWindow({ title, component, props = {}, w = 560, h = 420 }) {
    menuOpen = false
    const key = uid()
    customMeta.set(key, { component, props })
    windows = [
      ...windows,
      { key, id: `custom-${key}`, title, custom: true, x: 40, y: WORKSPACE_TOP, w, h, z: ++zTop },
    ]
  }

  setDesktop({ openCustomWindow })

  $effect(() => {
    if (authed) {
      api.me().then((u) => (user = u)).catch(() => logout())
      if (!openedDefault) {
        openedDefault = true
        openApp(APPS[0])
      }
    } else {
      openedDefault = false
    }
    const tick = () => {
      clock = new Date().toLocaleTimeString('fr-FR', { hour: '2-digit', minute: '2-digit' })
    }
    tick()
    const id = setInterval(tick, 30_000)
    return () => clearInterval(id)
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

  function openApp(app, offset = 0) {
    menuOpen = false
    const existing = windows.find((w) => w.id === app.id && !w.custom)
    if (existing) {
      focusWindow(existing.key)
      return
    }
    const key = uid()
    windows = [
      ...windows,
      {
        key,
        id: app.id,
        title: app.title,
        custom: false,
        x: 24 + offset * 20,
        y: WORKSPACE_TOP + offset * 24,
        w: app.w,
        h: app.h,
        z: ++zTop,
      },
    ]
  }

  function closeWindow(key) {
    windows = windows.filter((w) => w.key !== key)
    customMeta.delete(key)
  }

  function closeTopWindow() {
    if (!windows.length) return
    const top = windows.reduce((a, b) => (a.z > b.z ? a : b))
    closeWindow(top.key)
  }

  function focusWindow(key) {
    if (zTop >= 99) zTop = 10
    zTop++
    windows = windows.map((w) => (w.key === key ? { ...w, z: zTop } : w))
  }

  function onKeydown(e) {
    if (e.key === 'Escape') {
      e.preventDefault()
      closeTopWindow()
    }
  }

  function toggleMenu() {
    menuOpen = !menuOpen
  }
</script>

<svelte:window onclick={() => (menuOpen = false)} onkeydown={onKeydown} />

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
          <button class="task" class:active={w.z === Math.max(...windows.map((x) => x.z))} onclick={() => focusWindow(w.key)}>
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

    <div class="workspace">
      <div class="wallpaper"></div>
      <div class="icons">
        {#each APPS as app, i}
          <button class="icon" onclick={() => openApp(app, i)} title={app.title}>
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
              onclose={() => closeWindow(w.key)}
              onfocus={() => focusWindow(w.key)}
            >
              <svelte:component this={meta.component} {...meta.props} />
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
            onclose={() => closeWindow(w.key)}
            onfocus={() => focusWindow(w.key)}
          >
            <svelte:component this={APP_BY_ID[w.id].component} />
          </Window>
        {/if}
      {/each}
    </div>
  </div>
{/if}

<style>
  .desktop { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
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
  .workspace { position: relative; flex: 1; overflow: hidden; }
  .wallpaper {
    position: absolute; inset: 0;
    background: linear-gradient(160deg, #0c1f38 0%, #0a1628 40%, #061018 100%),
      url("data:image/svg+xml,%3Csvg width='60' height='60' xmlns='http://www.w3.org/2000/svg'%3E%3Cpath d='M0 30h60M30 0v60' stroke='%231a3050' stroke-width='0.5' opacity='0.3'/%3E%3C/svg%3E");
  }
  .icons { position: absolute; top: 16px; left: 16px; display: flex; flex-direction: column; gap: 8px; z-index: 1; }
  .icon {
    display: flex; flex-direction: column; align-items: center; gap: 4px; width: 76px;
    padding: 8px 4px; background: transparent; color: var(--bb-text); border-radius: 8px;
  }
  .icon:hover { background: rgba(255,255,255,0.08); }
  .emoji { font-size: 28px; filter: drop-shadow(0 2px 4px rgba(0,0,0,0.4)); }
  .label { font-size: 11px; text-align: center; text-shadow: 0 1px 3px rgba(0,0,0,0.8); line-height: 1.2; }
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
