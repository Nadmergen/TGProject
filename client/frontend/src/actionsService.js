import { DeleteMessage } from '../wailsjs/go/main/App';
import { chatHistory } from './stores';
import { selectedIds, clearSelection } from './selectionStore';
import { get } from 'svelte/store';

export const ActionsService = {
    async deleteSelected() {
        const ids = get(selectedIds);
        if (ids.length === 0) return;

        const confirmDelete = confirm(`Удалить выбранные сообщения (${ids.length})?`);
        if (!confirmDelete) return;

        try {
            for (let id of ids) {
                await DeleteMessage(Number(id));
                chatHistory.update(list => list.filter(m => m.id !== id));
            }
            clearSelection();
        } catch (err) {
            console.error("Delete error:", err);
            alert("Не удалось удалить сообщения: " + err);
        }
    }
};