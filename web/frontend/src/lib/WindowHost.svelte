<script>
  import { setWindowHost } from './windowHost.js'

  let { x, y, w, h, winKey, children } = $props()

  const host = $state({ x: 0, y: 0, w: 0, h: 0, key: '' })
  setWindowHost(host)

  $effect(() => {
    host.x = x
    host.y = y
    host.w = w
    host.h = h
    host.key = winKey
  })
</script>

{@render children()}
