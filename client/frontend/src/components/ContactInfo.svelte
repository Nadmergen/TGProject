<script>
    import { chatHistory, recipient } from '../stores';

    let activeTab = 'media';

    const hasLink = (text) =>
        typeof text === 'string' && /(https?:\/\/[^\s]+)/i.test(text);

    $: photos = $chatHistory.filter((m) => m.type === 'image');
    $: videos = $chatHistory.filter((m) => m.type === 'video');
    $: files = $chatHistory.filter((m) => m.type === 'file');
    $: voices = $chatHistory.filter((m) => m.type === 'voice');
    $: links = $chatHistory.filter((m) => m.type === 'text' && hasLink(m.content));

    function setTab(tab) {
        activeTab = tab;
    }
</script>

<aside class="info-root" aria-label="Информация о контакте">
    {#if $recipient}
        <div class="info-header">
            <div class="avatar">
                {#if $recipient.username}
                    {$recipient.username[0].toUpperCase()}
                {:else}
                    ?
                {/if}
            </div>
            <div class="title-block">
                <h3 class="name">{$recipient.username}</h3>
                <div class="subtitle">Профиль и медиа</div>
            </div>
        </div>

        <div class="tabs">
            <button
                type="button"
                class:active={activeTab === 'media'}
                on:click={() => setTab('media')}
            >
                Медиа
            </button>
            <button
                type="button"
                class:active={activeTab === 'files'}
                on:click={() => setTab('files')}
            >
                Файлы
            </button>
            <button
                type="button"
                class:active={activeTab === 'links'}
                on:click={() => setTab('links')}
            >
                Ссылки
            </button>
            <button
                type="button"
                class:active={activeTab === 'voice'}
                on:click={() => setTab('voice')}
            >
                Голосовые
            </button>
        </div>

        <div class="section-scroll">
            {#if activeTab === 'media'}
                <div class="grid">
                    {#if photos.length === 0 && videos.length === 0}
                        <div class="empty">Медиа пока нет</div>
                    {/if}

                    {#each photos as m (m.id)}
                        <div class="grid-item">
                            <img src={m.file_url} alt="Фото" />
                        </div>
                    {/each}

                    {#each videos as m (m.id)}
                        <div class="grid-item video">
                            <video src={m.file_url} controls>
                                <track kind="captions" />
                            </video>
                        </div>
                    {/each}
                </div>
            {:else if activeTab === 'files'}
                <ul class="list">
                    {#if files.length === 0}
                        <li class="empty">Файлов пока нет</li>
                    {/if}
                    {#each files as m (m.id)}
                        <li class="list-row">
                            <div class="icon">📄</div>
                            <div class="meta">
                                <div class="title">{m.file_name || 'Файл'}</div>
                                <a href={m.file_url} download={m.file_name}>Скачать</a>
                            </div>
                        </li>
                    {/each}
                </ul>
            {:else if activeTab === 'links'}
                <ul class="list">
                    {#if links.length === 0}
                        <li class="empty">Ссылок пока нет</li>
                    {/if}
                    {#each links as m (m.id)}
                        <li class="list-row">
                            <div class="icon">🔗</div>
                            <div class="meta">
                                <a
                                    href={m.content}
                                    target="_blank"
                                    rel="noreferrer"
                                >
                                    {m.content}
                                </a>
                            </div>
                        </li>
                    {/each}
                </ul>
            {:else if activeTab === 'voice'}
                <ul class="list">
                    {#if voices.length === 0}
                        <li class="empty">Голосовых сообщений пока нет</li>
                    {/if}
                    {#each voices as m (m.id)}
                        <li class="list-row">
                            <div class="icon">🎤</div>
                            <div class="meta">
                                <audio controls src={m.file_url}></audio>
                                <div class="title">{m.file_name || 'Голосовое сообщение'}</div>
                            </div>
                        </li>
                    {/each}
                </ul>
            {/if}
        </div>
    {:else}
        <div class="empty">Выберите чат</div>
    {/if}
</aside>

<style>
  .info-root {
    width: 320px;
    height: 100%;
    background: #17212b;
    border-left: 1px solid #080e13;
    display: flex;
    flex-direction: column;
  }

  .info-header {
    display: flex;
    align-items: center;
    padding: 16px;
    gap: 12px;
    border-bottom: 1px solid #080e13;
    background: #242f3d;
  }

  .avatar {
    width: 52px;
    height: 52px;
    border-radius: 50%;
    background: #4faeef;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    font-size: 20px;
    color: white;
    flex-shrink: 0;
  }

  .title-block {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .name {
    margin: 0;
    font-size: 16px;
    color: white;
  }

  .subtitle {
    font-size: 12px;
    color: #7f91a4;
  }

  .tabs {
    display: flex;
    padding: 6px 8px;
    gap: 6px;
    border-bottom: 1px solid #080e13;
  }

  .tabs button {
    flex: 1;
    background: none;
    border: none;
    padding: 6px 8px;
    border-radius: 999px;
    font-size: 12px;
    color: #7f91a4;
    cursor: pointer;
    transition: 0.2s;
  }

  .tabs button.active {
    background: #2b5278;
    color: white;
  }

  .section-scroll {
    flex: 1;
    overflow-y: auto;
    padding: 10px;
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 4px;
  }

  .grid-item img,
  .grid-item video {
    width: 100%;
    display: block;
    border-radius: 6px;
  }

  .grid-item.video {
    grid-column: span 3;
  }

  .list {
    list-style: none;
    padding: 0;
    margin: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .list-row {
    display: flex;
    gap: 10px;
    background: #0e1621;
    border-radius: 8px;
    padding: 8px 10px;
    border: 1px solid #080e13;
  }

  .icon {
    width: 28px;
    display: flex;
    align-items: center;
    justify-content: center;
    font-size: 18px;
  }

  .meta {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 4px;
  }

  .title {
    font-size: 13px;
    color: white;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }

  a {
    color: #4faeef;
    font-size: 12px;
    text-decoration: none;
  }

  a:hover {
    text-decoration: underline;
  }

  .empty {
    color: #7f91a4;
    font-size: 13px;
    padding: 16px;
    text-align: center;
  }
</style>

