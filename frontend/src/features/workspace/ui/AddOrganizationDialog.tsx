import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { ApiError } from "@/shared/api/client";
import { Button } from "@/shared/ui/button";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import { useCreateCompany } from "../api/workspace";

/**
 * Создание организации.
 *
 * Исправлено по сравнению со старой версией:
 *   — пустое имя больше не создаёт компанию без названия;
 *   — ошибка показывается, а не исчезает: старый код не имел .catch,
 *     и при отказе окно просто оставалось открытым.
 */
export function AddOrganizationDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const create = useCreateCompany();

  const submit = (event: FormEvent) => {
    event.preventDefault();
    const trimmed = name.trim();
    if (!trimmed) return;
    create.mutate(trimmed, { onSuccess: onClose });
  };

  const message =
    create.error instanceof ApiError && typeof create.error.body === "string"
      ? create.error.body
      : create.error?.message;

  return (
    <Modal onClose={onClose}>
      <form
        onSubmit={submit}
        className="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
      >
        <h2 className="text-base font-semibold">{t("workspace.addOrganization")}</h2>

        <Field label={t("workspace.organizationName")}>
          <Input autoFocus required value={name} onChange={(e) => setName(e.target.value)} />
        </Field>

        {message && (
          <p role="alert" className="rounded-md bg-danger-subtle px-3 py-2 text-sm text-danger">
            {message}
          </p>
        )}

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!name.trim() || create.isPending}>
            {t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
