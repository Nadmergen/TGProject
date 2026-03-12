import { writable } from 'svelte/store';

// Храним ID выбранных сообщений
export const selectedIds = writable([]);
// Режим выбора (включен/выключен)
export const isSelectionMode = writable(false);

export function toggleSelection(id) {
    selectedIds.update(ids => {
        if (ids.includes(id)) {
            return ids.filter(i => i !== id);
        } else {
            return [...ids, id];
        }
    });
}

export function clearSelection() {
    selectedIds.set([]);
    isSelectionMode.set(false);
}