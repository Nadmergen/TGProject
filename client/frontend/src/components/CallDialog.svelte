<script>
  export let open = false;
  export let onClose;

  // Add WebRTC logic here (e.g. initializing peer connection, handling stream)

  function endCall() {
    // Logic to end the call
    onClose();
  }
</script>

<template>
  {#if open}
    <div class="call-dialog">
      <h2>Incoming Call</h2>
      <video autoplay></video>
      <button on:click={endCall}>End Call</button>
      <button on:click={onClose}>Close</button>
    </div>
  {/if}
</template>

<style>
  .call-dialog {
    /* Basic styling for the dialog */
    background: white;
    border: 1px solid #ccc;
    padding: 20px;
    position: fixed;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    z-index: 1000;
  }
</style>
