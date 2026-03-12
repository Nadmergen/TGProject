<script>
  export let toggleInfo;

  let chatData = {
    id: "#001",
    type: "Публичный",
    mediaCount: "Медиа отсутствуют",
    description: "Группа для тестирования системы и взаимодействия пользователей."
  };

  let participants = [
    { name: "Администратор", role: "admin", color: "#ff9800", initial: "A" }
  ];
</script>

<aside class="info-panel">
  <header class="info-header">
    <button class="close-btn" on:click={toggleInfo}>✕</button>
    <h3>Информация</h3>
  </header>

  <div class="info-content">
    <div class="main-info">
      <div class="big-avatar">O</div>
      <h2>Общий чат</h2>
      <p class="status">онлайн</p>
    </div>

    <div class="info-section">
      <div class="info-row"><span class="label">ID чата:</span><span class="value">{chatData.id}</span></div>
      <div class="info-row"><span class="label">Тип:</span><span class="value">{chatData.type}</span></div>
      <div class="info-row"><span class="label">Медиа:</span><span class="value">{chatData.mediaCount}</span></div>
    </div>

    <div class="section">
       <h4>ОПИСАНИЕ</h4>
       <p class="desc">{chatData.description}</p>
    </div>

    <div class="section">
      <h4>УЧАСТНИКИ ({participants.length})</h4>
      <div class="p-list">
        {#each participants as user}
          <div class="p-item">
            <div class="p-avatar" style="background: {user.color}">{user.initial}</div>
            <div class="p-details">
              <span class="p-name">{user.name}</span>
              <span class="p-role">Администратор</span>
            </div>
          </div>
        {/each}
      </div>
    </div>
  </div>
</aside>

<style>
  .info-panel { width: 320px; background: #17212b; border-left: 1px solid #0e1621; display: flex; flex-direction: column; }
  .info-header { height: 56px; display: flex; align-items: center; padding: 0 20px; gap: 20px; border-bottom: 1px solid #0e1621; }
  .close-btn { background: none; border: none; color: #7f91a4; font-size: 20px; cursor: pointer; }
  .info-content { flex: 1; overflow-y: auto; padding: 20px; scrollbar-width: none; }
  .main-info { text-align: center; padding-bottom: 20px; border-bottom: 1px solid #242f3d; }
  .big-avatar { width: 80px; height: 80px; background: #2b5278; border-radius: 50%; margin: 0 auto 10px; display: flex; align-items: center; justify-content: center; font-size: 32px; }
  .info-row { display: flex; justify-content: space-between; margin-bottom: 10px; font-size: 14px; }
  .label { color: #7f91a4; }
  h4 { color: #4faeef; font-size: 12px; margin: 20px 0 10px; }
  .desc { font-size: 14px; color: #cfd8dc; line-height: 1.4; }
  .p-item { display: flex; align-items: center; gap: 12px; margin-bottom: 15px; }
  .p-avatar { width: 35px; height: 35px; border-radius: 50%; display: flex; align-items: center; justify-content: center; font-weight: bold; }
  .p-role { font-size: 11px; color: #7f91a4; display: block; }
</style>