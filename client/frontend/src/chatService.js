import { EventsOn } from '../wailsjs/runtime/runtime';
import { SendMessage, GetHistory, SelectFile, SetRecipient } from '../wailsjs/go/main/App';
import { chatHistory, currentUser, recipient } from './stores';
import { get } from 'svelte/store';

export async function initChat() {
    const history = await GetHistory();
    chatHistory.set(history || []);

    EventsOn("new_msg", (msg) => {
        chatHistory.update(list => [...list, msg]);
    });
}

// Вызывайте эту функцию при выборе собеседника
export function setActiveRecipient(userId) {
    SetRecipient(userId);
    recipient.set({ id: userId });
}

export async function handleUpload() {
    try {
        const fileInfo = await SelectFile();
        if (!fileInfo) return;

        const { name, url, is_image } = fileInfo;
        const type = is_image ? "image" : "file";

        // Отправляем сообщение (sender игнорируется, используется текущий пользователь)
        await SendMessage(
            get(currentUser)?.username || "",
            is_image ? "" : name,
            type,
            url,
            name
        );
    } catch (err) {
        console.error("Upload error:", err);
    }
}