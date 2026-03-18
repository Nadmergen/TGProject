<script>
  import { onDestroy } from 'svelte';
  import { callState, getLocalStream, getRemoteStream, hangup, toggleMute, toggleCamera, sendAnswer, acceptIncomingCall } from '../callService';

  let localVideo;
  let remoteVideo;

  function bindStreams() {
    const ls = getLocalStream();
    const rs = getRemoteStream();
    if (localVideo && ls) localVideo.srcObject = ls;
    if (remoteVideo && rs) remoteVideo.srcObject = rs;
  }

  $: bindStreams();

  async function accept() {
    await acceptIncomingCall();
    await sendAnswer();
  }

  onDestroy(() => {});
</script>

{#if $callState.incoming}
  <div class="call-overlay">
    <div class="call-card">
      <div class="title">Входящий { $callState.callType === 'video' ? 'видео' : 'голосовой' } звонок</div>
      <div class="subtitle">Пользователь: {$callState.peerUserId}</div>
      <div class="row">
        <button class="btn accept" on:click={accept}>Принять</button>
        <button class="btn hangup" on:click={hangup}>Отклонить</button>
      </div>
    </div>
  </div>
{/if}

{#if $callState.active}
  <div class="call-overlay">
    <div class="call-ui">
      <div class="videos { $callState.callType !== 'video' ? 'voice' : '' }">
        <video bind:this={remoteVideo} autoplay playsinline class="remote">
          <track kind="captions" />
        </video>
        <video bind:this={localVideo} autoplay muted playsinline class="local">
          <track kind="captions" />
        </video>
      </div>

      <div class="controls">
        <button class="btn" on:click={toggleMute}>{$callState.muted ? 'Вкл микрофон' : 'Выкл микрофон'}</button>
        {#if $callState.callType === 'video'}
          <button class="btn" on:click={toggleCamera}>{$callState.cameraOff ? 'Вкл камеру' : 'Выкл камеру'}</button>
        {/if}
        <button class="btn hangup" on:click={hangup}>Завершить</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .call-overlay {
    position: fixed;
    inset: 0;
    background: rgba(0,0,0,0.55);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
  }
  .call-card {
    width: 360px;
    background: #17212b;
    border: 1px solid #080e13;
    border-radius: 14px;
    padding: 18px;
    color: white;
  }
  .title { font-size: 16px; font-weight: 600; margin-bottom: 6px; }
  .subtitle { color: #b0b9c1; font-size: 13px; margin-bottom: 14px; }
  .row { display: flex; gap: 10px; }
  .btn {
    flex: 1;
    border: none;
    border-radius: 10px;
    padding: 10px 12px;
    cursor: pointer;
    background: #242f3d;
    color: white;
  }
  .btn.accept { background: #2e7d32; }
  .btn.hangup { background: #c62828; }
  .call-ui {
    width: min(900px, 92vw);
    background: #0e1621;
    border: 1px solid #080e13;
    border-radius: 16px;
    overflow: hidden;
  }
  .videos {
    position: relative;
    background: black;
    height: min(520px, 60vh);
  }
  .videos.voice {
    height: 200px;
  }
  video.remote { width: 100%; height: 100%; object-fit: cover; }
  video.local {
    position: absolute;
    right: 12px;
    bottom: 12px;
    width: 180px;
    height: 120px;
    object-fit: cover;
    border-radius: 12px;
    border: 1px solid rgba(255,255,255,0.2);
    background: rgba(0,0,0,0.4);
  }
  .controls {
    display: flex;
    gap: 10px;
    padding: 12px;
    background: #17212b;
  }
</style>

