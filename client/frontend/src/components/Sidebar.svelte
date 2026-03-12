<script>
  import { showDrawer, isSidebarOpen, innerWidth } from '../stores';
</script>

<aside class="sidebar-root">
  <div class="sidebar-top">
    <button class="burger-menu" on:click={() => showDrawer.set(true)}>☰</button>
    <div class="search-box">
      <input type="text" placeholder="🔍 Поиск" />
    </div>
  </div>

  <div class="items-container">
    <div class="chat-row active" on:click={() => { if($innerWidth < 750) isSidebarOpen.set(false) }}>
      <div class="chat-avatar-circle">O</div>
      <div class="chat-meta">
        <div class="chat-row-name">Общий чат</div>
        <div class="chat-row-last">Добро пожаловать!</div>
      </div>
    </div>
  </div>
</aside>

<style>
  .sidebar-root {
    width: 100%;
    height: 100%;
    background: #17212b;
    border-right: 1px solid #080e13;
    display: flex;
    flex-direction: column;
  }

  .sidebar-top {
    height: 56px;
    display: flex;
    align-items: center;
    padding: 0 12px;
    gap: 12px;
  }

  .burger-menu {
    background: none;
    border: none;
    color: #7f91a4;
    font-size: 22px;
    cursor: pointer;
    padding: 8px;
    border-radius: 8px;
    transition: 0.2s;
  }

  .burger-menu:hover {
    background: rgba(79, 174, 239, 0.1);
    color: #4faeef;
  }

  .search-box {
    flex: 1;
  }

  .search-box input {
    width: 100%;
    background: #242f3d;
    border: 1px solid #080e13;
    border-radius: 20px;
    padding: 8px 15px;
    color: white;
    outline: none;
    font-size: 14px;
    transition: 0.2s;
  }

  .search-box input:focus {
    border-color: #4faeef;
    background: #1f3347;
  }

  .search-box input::placeholder {
    color: #7f91a4;
  }

  .items-container {
    flex: 1;
    overflow-y: auto;
  }

  .chat-row {
    display: flex;
    padding: 10px 8px;
    gap: 12px;
    cursor: pointer;
    align-items: center;
    margin: 0 8px;
    border-radius: 8px;
    transition: 0.2s;
  }

  .chat-row:hover {
    background: rgba(79, 174, 239, 0.1);
  }

  .chat-row.active {
    background: #2b5278;
  }

  .chat-avatar-circle {
    width: 48px;
    height: 48px;
    background: #4faeef;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    color: white;
    flex-shrink: 0;
  }

  .chat-meta {
    flex: 1;
    min-width: 0;
  }

  .chat-row-name {
    font-weight: 500;
    color: white;
    font-size: 14px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .chat-row-last {
    font-size: 13px;
    color: #b0b9c1;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    margin-top: 2px;
  }
</style>