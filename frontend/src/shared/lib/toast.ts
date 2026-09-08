import { create } from "zustand";
import { errorText } from "../api/client";
import i18n, { type TranslationKey } from "./i18n";

/**
 * Уведомления.
 *
 * Нужны ровно ради одного: неудачный запрос не должен выглядеть как
 * ничего не произошло. Правка ячейки откатывалась молча, диалог поля
 * просто не закрывался — человек видел, что «не сохранилось», и не видел
 * почему. Текст ошибки знает только тот слой, где живёт запрос,
 * а показать его надо поверх всего экрана.
 *
 * Стор, а не контекст: уведомление зовётся из features/*\/api, то есть
 * из мутации, а не из компонента. `useToasts.getState()` работает
 * откуда угодно, хука для этого не требуется.
 */
export type ToastKind = "error" | "success";

export type Toast = {
  id: number;
  kind: ToastKind;
  text: string;
};

/**
 * Ошибку держим дольше: её читают, а не замечают краем глаза. Успех —
 * подтверждение того, что и так видно на экране.
 */
const LIFETIME: Record<ToastKind, number> = {
  error: 8000,
  success: 5000,
};

/** Больше трёх на экране — это уже не уведомление, а лог. */
const MAX = 3;

type ToastState = {
  items: Toast[];
  push: (kind: ToastKind, text: string) => void;
  dismiss: (id: number) => void;
};

let lastId = 0;

export const useToasts = create<ToastState>((set, get) => ({
  items: [],

  push: (kind, text) => {
    const id = ++lastId;

    set((state) => ({ items: [...state.items, { id, kind, text }].slice(-MAX) }));
    setTimeout(() => get().dismiss(id), LIFETIME[kind]);
  },

  dismiss: (id) => set((state) => ({ items: state.items.filter((item) => item.id !== id) })),
}));

export const toast = {
  error: (text: string) => useToasts.getState().push("error", text),
  success: (text: string) => useToasts.getState().push("success", text),
};

/**
 * Отказ запроса → уведомление. Один вызов на мутацию, поэтому и живёт
 * здесь, а не переписывается в каждой.
 *
 * Причину показываем ту, что прислал сервер, и только если он ничего
 * не сказал — свою общую формулировку. Перевод берётся из i18n напрямую:
 * мутация не компонент, хука `useTranslation` в ней нет.
 */
export function reportError(error: unknown, fallback: TranslationKey) {
  toast.error(errorMessage(error, fallback) ?? i18n.t(fallback));
}

/**
 * Отказ запроса → текст для ЭКРАНА, а не для уведомления.
 *
 * Тем же правилом, что и уведомление: причину показываем ту, что прислал
 * сервер, и только если он смолчал — свою. Отдельно от reportError,
 * потому что живут они по-разному: уведомление гаснет само, а на месте
 * несостоявшейся таблицы текст остаётся, пока запрос не повторят.
 *
 * null — ошибки нет. Так его и проверяют на экране.
 */
export function errorMessage(error: unknown, fallback: TranslationKey): string | null {
  if (!error) return null;
  return errorText(error) ?? i18n.t(fallback);
}
