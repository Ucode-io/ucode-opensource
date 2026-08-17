import { IconAlertTriangle, IconCheck, IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { useToasts } from "../lib/toast";
import { Icon } from "./icon";

/**
 * Стопка уведомлений поверх экрана. Один экземпляр на приложение —
 * в корневом маршруте.
 *
 * Внизу справа: сверху живёт шапка таблицы, слева — сайдбар, а правый
 * нижний угол свободен на всех экранах и не перекрывает строку, которую
 * человек только что правил.
 *
 * role="status" и aria-live: без них уведомление увидят только глазами,
 * а сообщение об ошибке — единственный след неудачного сохранения.
 */
const STYLES = {
  error: "border-danger bg-danger-subtle text-danger",
  success: "border-success bg-success-subtle text-success",
};

const ICONS = {
  error: IconAlertTriangle,
  success: IconCheck,
};

export function Toaster() {
  const { t } = useTranslation();
  const items = useToasts((state) => state.items);
  const dismiss = useToasts((state) => state.dismiss);

  if (!items.length) return null;

  return (
    <div
      role="status"
      aria-live="polite"
      className="pointer-events-none fixed right-4 bottom-4 z-100 flex w-80 flex-col gap-2"
    >
      {items.map((item) => (
        <div
          key={item.id}
          className={`pointer-events-auto flex items-start gap-2 rounded-lg border p-3 shadow-popover ${STYLES[item.kind]}`}
        >
          <Icon as={ICONS[item.kind]} size={16} className="mt-0.5" />

          {/* Текст ошибки приходит с сервера и бывает длинным: переносим,
              а не обрезаем — обрезанная причина бесполезна. */}
          <p className="min-w-0 flex-1 text-sm break-words">{item.text}</p>

          <button
            type="button"
            onClick={() => dismiss(item.id)}
            aria-label={t("action.close")}
            className="-m-1 grid size-6 shrink-0 place-items-center rounded-md opacity-60 transition-opacity hover:opacity-100"
          >
            <Icon as={IconX} size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}
