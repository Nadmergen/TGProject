<script>
    import { onMount } from 'svelte';
    import { showDrawer, isSidebarOpen, innerWidth, recipient, currentUser } from '../stores';
    import { setActiveRecipient } from '../chatService';
    import { API_URL } from '../config';

    let search = '';
    let chats = [];
    let isLoading = false;
    let loadError = '';

    function openChat(chat) {
        setActiveRecipient(chat.id, chat.title);
        if ($innerWidth < 750) isSidebarOpen.set(false);
    }

    function getInitials(title) {
        if (!title) return '?';
        const parts = title.trim().split(' ');
        if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
        return (parts[0][0] + parts[1][0]).toUpperCase();
    }

    async function loadChats() {
        isLoading = true;
        loadError = '';
        try {
            const token = localStorage.getItem('token');
            const res = await fetch(`${API_URL}/api/contacts`, {
                headers: {
                    'Authorization': `Bearer ${token}`,
                    'X-User-ID': $currentUser?.id?.toString() || ''
                }
            });
            const data = await res.json();

            if (!res.ok) {
                throw new Error(data.error || 'Не удалось загрузить список чатов');
            }

            // Преобразуем контакты в список чатов + добавляем "Избранное"
            const contactChats = (data || []).map((c) => ({
                id: c.id,
                title: c.name,
                lastMessage: c.last_message || '',
            }));

            chats = [
                {
                    id: $currentUser?.id,
                    title: 'Избранное',
                    lastMessage: '',
                },
                ...contactChats,
            ];
        } catch (e) {
            loadError = 'Ошибка загрузки чатов';
        }
        isLoading = false;
    }

    const filteredChats = () => {
        if (!search.trim()) return chats;
        const q = search.trim().toLowerCase();
        return chats.filter((c) => c.title.toLowerCase().includes(q));
    };

    onMount(() => {
        loadChats();
    });
</script>

<aside class="sidebar-root">
    <div class="sidebar-top">
        <button class="burger-menu" on:click={() => showDrawer.set(true)} aria-label="Меню">☰</button>
        <div class="search-box">
            <span class="search-icon">🔍</span>
            <input
                type="text"
                bind:value={search}
                placeholder="Поиск"
            />
        </div>
    </div>

    <div class="items-container">
        {#if isLoading}
            <div class="hint">Загрузка чатов...</div>
        {:else if loadError}
            <div class="hint error">{loadError}</div>
        {:else if filteredChats().length === 0}
            <div class="hint">Ничего не найдено</div>
        {:else}
            {#each filteredChats() as chat}
                <button
                    type="button"
                    class="chat-row { $recipient && $recipient.id === chat.id ? 'active' : '' }"
                    on:click={() => openChat(chat)}
                >
                    <div class="chat-avatar-circle">
                        {getInitials(chat.title)}
                    </div>
                    <div class="chat-meta">
                        <div class="chat-row-name">{chat.title}</div>
                        {#if chat.lastMessage}
                            <div class="chat-row-last">{chat.lastMessage}</div>
                        {/if}
                    </div>
                </button>
            {/each}
        {/if}
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
    padding: 0 8px;
    gap: 8px;
    border-bottom: 1px solid #080e13;
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
    position: relative;
  }

  .search-box input {
    width: 100%;
    background: #0e1621;
    border: none;
    border-radius: 18px;
    padding: 8px 14px 8px 30px;
    color: white;
    outline: none;
    font-size: 13px;
    transition: 0.2s;
    box-shadow: inset 0 0 0 1px #080e13;
    box-sizing: border-box;
  }

  .search-box input:focus {
    box-shadow: inset 0 0 0 1px #4faeef;
    background: #17212b;
  }

  .search-box input::placeholder {
    color: #7f91a4;
  }

  .search-icon {
    position: absolute;
    left: 10px;
    top: 50%;
    transform: translateY(-50%);
    font-size: 13px;
    color: #7f91a4;
    pointer-events: none;
  }

  .items-container {
    flex: 1;
    overflow-y: auto;
  }

  .hint {
    padding: 12px 16px;
    color: #7f91a4;
    font-size: 13px;
  }

  .hint.error {
    color: #ef5350;
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