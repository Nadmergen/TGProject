import { get } from 'svelte/store';
import { chatHistory, currentUser, recipient } from './stores';
import { EventsOn } from '../wailsjs/runtime/runtime';

import { API_URL } from './config';
import { setCallWebSocket, handleSignalingMessage } from './callService';

// Инициализация WebSocket (через Wails, но пока не используется)
export function initChat() {
    const token = localStorage.getItem('token');
    if (!token) return;

    try {
        const wsURL = API_URL.replace(/^http/, 'ws') + `/ws?token=${encodeURIComponent(token)}`;
        const ws = new WebSocket(wsURL);
        setCallWebSocket(ws);
        ws.onmessage = (evt) => {
            try {
                const msg = JSON.parse(evt.data);
                if (!msg || !msg.event) return;
                if (msg.event.startsWith('call_')) {
                    handleSignalingMessage(msg);
                    return;
                }
                if (msg.event === 'message_delivered') {
                    chatHistory.update(h => h.map(m => m.id === msg.id ? { ...m, delivered_at: msg.delivered_at } : m));
                } else if (msg.event === 'message_read') {
                    chatHistory.update(h => h.map(m => m.id === msg.id ? { ...m, read_at: msg.read_at } : m));
                } else if (msg.event === 'message_created') {
                    // New incoming message for current user.
                    chatHistory.update(h => [...h, msg]);
                }
            } catch (_) {}
        };
    } catch (e) {
        console.warn('WS init failed', e);
    }
}

// Установка активного получателя
export function setActiveRecipient(userId, username) {
    recipient.set({ id: userId, username });
    // Можно загрузить историю сообщений с этим пользователем
    loadMessages(userId);
}

// Загрузка истории сообщений с конкретным пользователем
export async function loadMessages(contactId) {
    const token = localStorage.getItem('token');
    if (!token) return;

    try {
        const res = await fetch(`${API_URL}/api/messages?page=1`, {
            headers: {
                'Authorization': `Bearer ${token}`,
                'X-User-ID': get(currentUser)?.id?.toString() || ''
            }
        });
        const data = await res.json();
        const resolved = await Promise.all((data || []).map(async (m) => {
            const fileKey = typeof m.file_url === 'string' ? m.file_url : '';
            if (fileKey.startsWith('uploads/')) {
                const dl = await getDownloadURL(fileKey).catch(() => '');
                return { ...m, object_key: fileKey, file_url: dl || fileKey };
            }
            return m;
        }));
        // Фильтруем сообщения, относящиеся к текущему контакту
        const filtered = resolved.filter(m =>
            (m.sender_id === contactId && m.recipient_id === get(currentUser)?.id) ||
            (m.sender_id === get(currentUser)?.id && m.recipient_id === contactId)
        );
        chatHistory.set(filtered);
    } catch (e) {
        console.error('Error loading messages', e);
    }
}

export async function getDownloadURL(objectKey) {
    const token = localStorage.getItem('token');
    if (!token) throw new Error('No token');
    const res = await fetch(`${API_URL}/api/uploads/download?object_key=${encodeURIComponent(objectKey)}`, {
        headers: { 'Authorization': `Bearer ${token}` }
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to get download url');
    return data.download_url;
}

// Отправка текстового сообщения
export async function sendTextMessage(content) {
    const target = get(recipient);
    const user = get(currentUser);
    if (!target || !user) return;

    const token = localStorage.getItem('token');
    const res = await fetch(`${API_URL}/api/messages/send`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
            'X-User-ID': user.id.toString()
        },
        body: JSON.stringify({
            recipient_id: target.id,
            content: content,
            type: 'text',
            file_url: '',
            file_name: ''
        })
    });
    const data = await res.json().catch(() => ({}));
    if (!res.ok) {
        throw new Error(data.error || data.message || 'Failed to send message');
    }
    if (res.ok) {
        // Оптимистично добавляем в историю (можно дождаться WebSocket)
        chatHistory.update(h => [...h, {
            id: data.id,
            sender_id: user.id,
            recipient_id: target.id,
            content,
            type: 'text',
            file_url: '',
            file_name: '',
            created_at: new Date().toISOString()
        }]);
    }
    return data;
}

async function initUpload(file) {
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_URL}/api/uploads/init`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
            file_name: file.name,
            content_type: file.type || 'application/octet-stream',
            size: file.size
        })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to init upload');
    return data; // {object_key, upload_url}
}

async function putToS3(uploadURL, file) {
    const res = await fetch(uploadURL, {
        method: 'PUT',
        headers: { 'Content-Type': file.type || 'application/octet-stream' },
        body: file
    });
    if (!res.ok) throw new Error('Failed to upload to storage');
}

async function completeUpload(targetId, file, objectKey, msgType, content = '') {
    const token = localStorage.getItem('token');
    const res = await fetch(`${API_URL}/api/uploads/complete`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`
        },
        body: JSON.stringify({
            recipient_id: targetId,
            type: msgType,
            content,
            object_key: objectKey,
            file_name: file.name,
            content_type: file.type || 'application/octet-stream',
            size: file.size
        })
    });
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || 'Failed to save message');
    return data; // {id,status,file_url,download_url}
}

function detectMessageType(file) {
    const t = (file?.type || '').toLowerCase();
    if (t.startsWith('video/')) return 'video';
    if (t.startsWith('image/')) return 'image';
    if (t.startsWith('audio/')) return 'voice';
    return 'file';
}

export async function sendAttachment(file) {
    const target = get(recipient);
    const user = get(currentUser);
    if (!target || !user) return;

    const msgType = detectMessageType(file);
    const { object_key, upload_url } = await initUpload(file);
    await putToS3(upload_url, file);
    const saved = await completeUpload(target.id, file, object_key, msgType);

    chatHistory.update(h => [...h, {
        id: saved.id,
        sender_id: user.id,
        recipient_id: target.id,
        content: '',
        type: msgType,
        object_key: object_key,
        file_url: saved.download_url || object_key,
        file_name: file.name,
        created_at: new Date().toISOString()
    }]);
    return saved;
}

// Отправка голосового сообщения (после загрузки)
export async function sendVoiceMessage(file) {
    try {
        return await sendAttachment(file);
    } catch (e) {
        console.error('Attachment send error', e);
    }
}

// Удаление сообщения
export async function deleteMessage(id) {
    const token = localStorage.getItem('token');
    const user = get(currentUser);
    if (!user) return;

    const res = await fetch(`${API_URL}/api/messages/delete`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
            'Authorization': `Bearer ${token}`,
            'X-User-ID': user.id.toString()
        },
        body: JSON.stringify({ id })
    });
    if (res.ok) {
        chatHistory.update(h => h.filter(m => m.id !== id));
    }
    return res.json();
}