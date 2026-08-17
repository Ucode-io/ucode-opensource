import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";

/** Подтверждение необратимого действия. Кнопка называет действие, а не «ОК». */
export function ConfirmDialog({
  title,
  description,
  confirmLabel,
  busy,
  onConfirm,
  onClose,
}: {
  title: string;
  description: string;
  confirmLabel: string;
  busy: boolean;
  onConfirm: () => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center p-4"
      style={{ background: "var(--color-overlay)" }}
      onPointerDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <div className="flex w-full max-w-sm flex-col gap-3 rounded-xl border border-border bg-surface p-5 shadow-modal">
        <h2 className="text-base font-semibold">{title}</h2>
        <p className="text-sm text-fg-muted">{description}</p>

        <div className="mt-1 flex justify-end gap-2">
          <Button variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button variant="danger" disabled={busy} onClick={onConfirm} className="border border-danger">
            {confirmLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}
