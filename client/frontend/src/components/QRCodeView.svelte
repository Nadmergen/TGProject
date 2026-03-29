<script>
  import QRCode from 'qrcode';
  import { onMount } from 'svelte';

  export let text = '';

  let canvasEl;
  let error = '';

  async function render() {
    if (!canvasEl || !text) return;
    error = '';
    try {
      await QRCode.toCanvas(canvasEl, text, {
        width: 220,
        margin: 2,
        color: {
          dark: '#ffffff',
          light: '#0e1621'
        }
      });
    } catch (e) {
      error = 'Не удалось отрисовать QR';
    }
  }

  onMount(render);

  $: if (text) {
    render();
  }
</script>

<div class="qr-wrapper">
  {#if text}
    <canvas bind:this={canvasEl} aria-label="QR код для входа"></canvas>
  {:else}
    <div class="placeholder">QR будет здесь</div>
  {/if}
  {#if error}
    <div class="err">{error}</div>
  {/if}
</div>

<style>
  .qr-wrapper {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }

  canvas {
    background: #0e1621;
    border-radius: 16px;
    padding: 10px;
    box-shadow: 0 10px 25px rgba(0,0,0,0.4);
  }

  .placeholder {
    padding: 20px;
    font-size: 13px;
    color: #7f91a4;
  }

  .err {
    font-size: 12px;
    color: #ef5350;
  }
</style>

