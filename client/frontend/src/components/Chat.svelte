<script>
  import { afterUpdate, tick } from 'svelte';
  import { chatHistory, currentUser } from '../stores';
  import { isSelectionMode, selectedIds, clearSelection } from '../selectionStore';
  import { SendMessage } from '../../wailsjs/go/main/App';
  import { handleUpload } from '../chatService';
  import Message from './Message.svelte';

  export let toggleInfo;
  let text = "";
  let scrollBox;

  async function handleSend() {
    if (!text.trim()) return;
    const name = $currentUser?.username || "Аноним";
    await SendMessage(name, text, "text", "", "");
    text = "";
    await tick();
    if (scrollBox) scrollBox.scrollTop = scrollBox.scrollHeight;
  }

  async function handleBulkDelete() {
    if (confirm(`Удалить ${$selectedIds.length} сообщений?`)) {
      const { DeleteMessage } = await import('../../wailsjs/go/main/App');
      for (let id of $selectedIds) {
        await DeleteMessage(id);
      }
      chatHistory.update(list => list.filter(m => !$selectedIds.includes(m.id)));
      clearSelection();
    }
  }

  function handleForward() {
    alert(`Пересылка сообщений: ${$selectedIds.join(', ')}\n(Здесь откроется окно выбора контакта)`);
    clearSelection();
  }

  afterUpdate(() => {
    if (scrollBox && !$isSelectionMode) scrollBox.scrollTop = scrollBox.scrollHeight;
  });
</script>

<div class="chat-content">
  {#if $isSelectionMode && $selectedIds.length > 0}
    <header class="chat-header selection-header">
      <div class="sel-left">
        <button class="icon-btn" on:click={clearSelection}>✕</button>
        <span class="sel-count">Выбрано: {$selectedIds.length}</span>
      </div>
      <div class="sel-actions">
        <button class="action-btn" on:click={handleForward}>➔ Переслать</button>
        <button class="action-btn danger" on:click={handleBulkDelete}>🗑 Удалить</button>
      </div>
    </header>
  {:else}
    <header class="chat-header" on:click={toggleInfo}>
      <div class="chat-info-block">
        <h2 class="chat-title">Общий чат</h2>
        <span class="chat-status">онлайн</span>
      </div>
      <button class="info-btn" title="Информация">
        <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <circle cx="12" cy="12" r="10"></circle>
          <line x1="12" y1="16" x2="12" y2="12"></line>
          <line x1="12" y1="8" x2="12.01" y2="8"></line>
        </svg>
      </button>
    </header>
  {/if}

  <div class="messages-area" bind:this={scrollBox}>
    {#if $chatHistory && $chatHistory.length > 0}
      {#each $chatHistory as msg (msg.id || Math.random())}
        <Message {msg} />
      {/each}
    {:else}
      <div class="empty">Сообщений пока нет...</div>
    {/if}
  </div>

  <div class="input-container">
    <button class="icon-btn attach-btn" on:click={handleUpload} title="Прикрепить файл">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 0 19.8-4.3M22 5.5a10 10 0 0 0-19.8 4.2"></path>
      </svg>
    </button>
    <input
      type="text"
      bind:value={text}
      on:keydown={(e) => e.key === 'Enter' && handleSend()}
      placeholder="Написать сообщение..."
      class="message-input"
    />
    <button class="send-btn" on:click={handleSend} disabled={!text.trim()} title="Отправить (Enter)">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor">
        <path d="M16.6915026,12.4744748 L3.50612381,13.2599618 C3.19218622,13.2599618 3.03521743,13.4170592 3.03521743,13.5741566 L1.15159189,20.0151496 C0.8376543,20.8006365 0.99,21.89 1.77946707,22.52 C2.41,22.99 3.50612381,23.1 4.13399899,22.8429026 L21.714504,14.0454487 C22.6563168,13.5741566 23.1272231,12.6315722 22.9702544,11.6889879 L4.13399899,1.16770959 C3.34915502,0.9106122 2.40734225,1.0177098 1.77946707,1.4890019 C0.994623095,2.1605983 0.837654326,3.10604706 1.15159189,3.89154405 L3.03521743,10.3325371 C3.03521743,10.4896346 3.19218622,10.646732 3.50612381,10.646732 L16.6915026,11.4322189 C16.6915026,11.4322189 17.1624089,11.4322189 17.1624089,12.0038152 C17.1624089,12.4744748 16.6915026,12.4744748 16.6915026,12.4744748 Z"></path>
      </svg>
    </button>
  </div>
</div>

<style>
  .chat-content { display: flex; flex-direction: column; height: 100%; width: 100%; background: #0e1621; }

  .chat-header {
    height: 56px; background: #17212b; display: flex; align-items: center; justify-content: space-between;
    padding: 0 15px; border-bottom: 1px solid #080e13; cursor: pointer; flex-shrink: 0;
  }
  .selection-header { background: #2b5278; cursor: default; }

  .sel-left { display: flex; align-items: center; gap: 15px; }
  .sel-count { font-weight: bold; font-size: 16px; color: white; }
  .sel-actions { display: flex; gap: 10px; }
  .action-btn { background: none; border: none; color: white; cursor: pointer; font-size: 14px; font-weight: bold; }
  .action-btn.danger { color: #ff5252; }

  .chat-info-block { flex: 1; min-width: 0; }
  .chat-title { margin: 0; font-size: 16px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: white; }
  .chat-status { font-size: 12px; color: #4faeef; }

  .info-btn {
    background: none;
    border: none;
    color: #7f91a4;
    cursor: pointer;
    padding: 8px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: 0.2s;
  }

  .info-btn:hover {
    color: #4faeef;
    background: rgba(79, 174, 239, 0.1);
  }

  .messages-area {
    flex: 1; overflow-y: auto; padding: 10px 0; display: flex; flex-direction: column;
    scrollbar-width: thin; scrollbar-color: rgba(255,255,255,0.1) transparent;
  }
  .messages-area::-webkit-scrollbar { width: 5px; }
  .messages-area::-webkit-scrollbar-thumb { background: rgba(255, 255, 255, 0.1); border-radius: 5px; }

  .input-container {
    background: #17212b;
    padding: 12px 15px;
    display: flex;
    align-items: center;
    gap: 10px;
    border-top: 1px solid #080e13;
  }

  .attach-btn {
    background: none;
    border: none;
    color: #7f91a4;
    cursor: pointer;
    padding: 8px;
    border-radius: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: 0.2s;
  }

  .attach-btn:hover {
    color: #4faeef;
    background: rgba(79, 174, 239, 0.1);
  }

  .message-input {
    flex: 1;
    background: #242f3d;
    border: 1px solid #080e13;
    color: white;
    border-radius: 20px;
    padding: 10px 16px;
    outline: none;
    font-size: 14px;
    transition: 0.2s;
  }

  .message-input:focus {
    border-color: #4faeef;
    box-shadow: 0 0 0 2px rgba(79, 174, 239, 0.1);
  }

  .send-btn {
    background: #4faeef;
    border: none;
    color: white;
    width: 36px;
    height: 36px;
    border-radius: 50%;
    cursor: pointer;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: 0.2s;
    flex-shrink: 0;
  }

  .send-btn:hover:not(:disabled) {
    background: #3b90d1;
  }

  .send-btn:disabled {
    background: #4faeef;
    opacity: 0.4;
    cursor: not-allowed;
  }

  .empty {
    text-align: center;
    color: #7f91a4;
    margin: auto;
  }
</style>