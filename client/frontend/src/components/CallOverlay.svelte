<script>
  import { onDestroy } from 'svelte';
  import { callState, getLocalStream, getRemoteStream, hangup, toggleMute, toggleCamera, enableVideo, sendAnswer, acceptIncomingCall } from '../callService';

  let localVideo;
  let remoteVideo;
  let raf = 0;
  let audioCtx;
  let localLevel = 0;
  let remoteLevel = 0;

  function bindStreams() {
    const ls = getLocalStream();
    const rs = getRemoteStream();
    if (localVideo && ls) localVideo.srcObject = ls;
    if (remoteVideo && rs) remoteVideo.srcObject = rs;
  }

  $: bindStreams();

  function stopMeters() {
    if (raf) cancelAnimationFrame(raf);
    raf = 0;
    try { audioCtx?.close(); } catch (_) {}
    audioCtx = null;
    localLevel = 0;
    remoteLevel = 0;
  }

  function startMeters() {
    stopMeters();
    const ls = getLocalStream();
    const rs = getRemoteStream();
    if (!ls && !rs) return;

    audioCtx = new (window.AudioContext || window.webkitAudioContext)();

    const mkMeter = (stream) => {
      if (!stream) return null;
      const hasAudio = stream.getAudioTracks().length > 0;
      if (!hasAudio) return null;
      const src = audioCtx.createMediaStreamSource(stream);
      const analyser = audioCtx.createAnalyser();
      analyser.fftSize = 256;
      src.connect(analyser);
      const data = new Uint8Array(analyser.frequencyBinCount);
      return { analyser, data };
    };

    const lm = mkMeter(ls);
    const rm = mkMeter(rs);

    const loop = () => {
      const calc = (m) => {
        if (!m) return 0;
        m.analyser.getByteTimeDomainData(m.data);
        let sum = 0;
        for (let i = 0; i < m.data.length; i++) {
          const v = (m.data[i] - 128) / 128;
          sum += v * v;
        }
        const rms = Math.sqrt(sum / m.data.length);
        return Math.min(1, rms * 2.2);
      };

      localLevel = calc(lm);
      remoteLevel = calc(rm);
      raf = requestAnimationFrame(loop);
    };

    raf = requestAnimationFrame(loop);
  }

  $: if ($callState.active) {
    // restart meters when call starts or upgrades to video
    startMeters();
  } else {
    stopMeters();
  }

  async function accept() {
    await acceptIncomingCall();
    await sendAnswer();
  }

  onDestroy(() => {
    stopMeters();
  });
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
      {#if $callState.callType === 'video'}
        <div class="videos">
          <video bind:this={remoteVideo} autoplay playsinline class="remote">
            <track kind="captions" />
          </video>
          <video bind:this={localVideo} autoplay muted playsinline class="local">
            <track kind="captions" />
          </video>
        </div>
      {:else}
        <div class="voice-ui">
          <div class="voice-avatar">
            {#if $callState.peerUsername}
              {$callState.peerUsername[0].toUpperCase()}
            {:else}
              ?
            {/if}
          </div>
          <div class="voice-title">Голосовой звонок</div>
        </div>
      {/if}

      <div class="controls">
        <button class="btn" on:click={toggleMute}>
          {$callState.muted ? 'Вкл микрофон' : 'Выкл микрофон'}
          <span class="meter" title="Уровень микрофона">
            <span class="bar" style="width: {Math.round(localLevel * 100)}%"></span>
          </span>
        </button>
        {#if $callState.callType === 'video'}
          <button class="btn" on:click={toggleCamera}>{$callState.cameraOff ? 'Вкл камеру' : 'Выкл камеру'}</button>
        {:else}
          <button class="btn" on:click={enableVideo}>Включить видео</button>
        {/if}
        <div class="remote-meter" title="Уровень входящего звука">
          <span class="label">Звук</span>
          <span class="meter">
            <span class="bar" style="width: {Math.round(remoteLevel * 100)}%"></span>
          </span>
        </div>
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
    align-items: center;
  }

  .meter {
    display: inline-flex;
    width: 70px;
    height: 6px;
    margin-left: 10px;
    border-radius: 999px;
    background: rgba(255,255,255,0.12);
    overflow: hidden;
    vertical-align: middle;
  }
  .meter .bar {
    height: 100%;
    background: #4faeef;
    width: 0%;
  }
  .remote-meter {
    display: flex;
    align-items: center;
    gap: 8px;
    color: #b0b9c1;
    font-size: 12px;
    padding: 0 6px;
    border-radius: 10px;
    background: rgba(0,0,0,0.15);
    height: 36px;
  }
  .remote-meter .label { opacity: 0.9; }

  .voice-ui {
    height: 260px;
    background: #0b111a;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
  }

  .voice-avatar {
    width: 96px;
    height: 96px;
    border-radius: 50%;
    background: #4faeef;
    display: flex;
    align-items: center;
    justify-content: center;
    color: white;
    font-size: 34px;
    font-weight: 700;
  }

  .voice-title {
    color: #b0b9c1;
    font-size: 14px;
  }
</style>

