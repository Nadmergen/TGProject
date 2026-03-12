<script>
  import { currentUser, chatHistory } from '../stores';
  import { isSelectionMode, selectedIds, toggleSelection } from '../selectionStore';
  import { DeleteMessage } from '../../wailsjs/go/main/App';

  export let msg;

  $: isMine = $currentUser && msg.sender === $currentUser.username;
  $: msgId = msg.id;
  $: isSelected = $selectedIds.includes(msgId);

  async function handleDelete() {
    if (!confirm("Удалить сообщение?")) return;
    const result = await DeleteMessage(msgId);
    if (result === "success") {
      chatHistory.update(list => list.filter(m => m.id !== msgId));
    }
  }

  function handleClick() {
    if ($isSelectionMode) {
      toggleSelection(msgId);
    }
  }

  function handleLongPress(e) {
    e.preventDefault();
    $isSelectionMode = true;
    toggleSelection(msgId);
  }

  // Определяем видимость галочек
  $: showTicks = isMine ? (msg.read ? '✓✓' : '✓') : null;
  $: timeLabel = msg.readAt ? `просмотрено ${msg.readAt}` : '';
</script>

<div
  class="msg-row {isMine ? 'is-mine' : ''} {isSelected ? 'selected' : ''}"
  on:click={handleClick}
  on:contextmenu={handleLongPress}
  role="button"
  tabindex="0"
>
  {#if $isSelectionMode}
    <div class="select-indicator {isSelected ? 'active' : ''}">
      {#if isSelected}
        <svg viewBox="0 0 24 24" fill="white">
          <path d="M9 16.17L4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41L9 16.17z"/>
        </svg>
      {/if}
    </div>
  {/if}

  <div class="msg-bubble">
    {#if !isMine}
      <div class="sender-name">{msg.sender || 'Аноним'}</div>
    {/if}

    <div class="msg-content">
      {#if msg.type === 'image'}
        <img src={msg.file_url} alt="Message" class="chat-img" on:contextmenu={handleLongPress} />
      {:else if msg.type === 'file'}
        <div class="file-attach">
          <span class="file-icon">📄</span>
          <div class="file-details">
            <span class="file-name">{msg.file_name || 'Файл'}</span>
            <a href={msg.file_url} download={msg.file_name} class="dl-link">Скачать</a>
          </div>
        </div>
      {:else}
        <p class="text">{msg.content}</p>
      {/if}
    </div>

    <div class="msg-footer">
      <span class="time">{msg.time || '12:00'}</span>
      {#if showTicks}
        <span class="ticks {msg.read ? 'read' : ''}">{showTicks}</span>
      {/if}
    </div>

    {#if timeLabel}
      <div class="read-label">{timeLabel}</div>
    {/if}
  </div>
</div>

<style>
  .msg-row {
    display: flex;
    width: 100%;
    padding: 4px 15px;
    cursor: pointer;
    align-items: flex-end;
    gap: 10px;
    box-sizing: border-box;
    position: relative;
    transition: 0.15s background;
  }

  .msg-row.selected {
    background: rgba(79, 174, 239, 0.1);
  }

  .msg-row:hover {
    background: rgba(79, 174, 239, 0.05);
  }

  .is-mine {
    justify-content: flex-end;
  }

  .msg-bubble {
    max-width: 70%;
    padding: 8px 12px;
    border-radius: 15px;
    background: #182533;
    color: white;
    position: relative;
  }

  .is-mine .msg-bubble {
    background: #2b5278;
    border-bottom-right-radius: 4px;
  }

  .msg-row:not(.is-mine) .msg-bubble {
    border-bottom-left-radius: 4px;
  }

  .sender-name {
    color: #4faeef;
    font-size: 13px;
    font-weight: bold;
    margin-bottom: 4px;
  }

  .text {
    margin: 0;
    font-size: 15px;
    white-space: pre-wrap;
    line-height: 1.4;
    word-break: break-word;
  }

  .msg-footer {
    display: flex;
    justify-content: flex-end;
    gap: 4px;
    margin-top: 4px;
    opacity: 0.6;
    font-size: 11px;
  }

  .ticks {
    color: #4faeef;
  }

  .ticks.read {
    color: #4faeef;
    font-weight: bold;
  }

  .read-label {
    font-size: 10px;
    color: #7f91a4;
    margin-top: 2px;
    opacity: 0.7;
  }

  .select-indicator {
    width: 22px;
    height: 22px;
    border: 2px solid #7f91a4;
    border-radius: 50%;
    flex-shrink: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: 0.2s;
  }

  .select-indicator.active {
    background: #4faeef;
    border-color: #4faeef;
  }

  .select-indicator svg {
    width: 14px;
    height: 14px;
  }

  .chat-img {
    max-width: 100%;
    border-radius: 8px;
    cursor: pointer;
    transition: 0.2s;
  }

  .chat-img:hover {
    opacity: 0.9;
  }

  .file-attach {
    display: flex;
    align-items: center;
    gap: 8px;
    background: rgba(0,0,0,0.2);
    padding: 8px;
    border-radius: 8px;
  }

  .file-icon {
    font-size: 16px;
  }

  .file-details {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }

  .file-name {
    font-size: 13px;
    font-weight: 500;
    color: white;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  .dl-link {
    color: #4faeef;
    text-decoration: none;
    font-weight: bold;
    font-size: 11px;
  }

  .dl-link:hover {
    text-decoration: underline;
  }
</style>