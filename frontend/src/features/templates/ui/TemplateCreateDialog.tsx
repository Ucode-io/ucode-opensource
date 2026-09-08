import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { localized, useTables } from "@/features/table";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import { useCreateTemplate } from "../api/templates";

/**
 * «Сохранить как шаблон» из папки: имя, описание и отмеченные таблицы.
 *
 * Больше от нас ничего не нужно — поля, связи, view, раскладки, действия
 * и дерево пунктов шлюз дочитывает сам (см. api/templates, useCreateTemplate).
 * Старая админка показывала здесь ещё вкладки функций и микрофронтендов;
 * оба ключа шлюз кладёт в шаблон как есть, но собирать их было неоткуда —
 * списки в её форме всегда оставались пустыми.
 *
 * Список таблиц — весь проект, а не только содержимое папки: таблица
 * не обязана лежать под тем же пунктом, из которого делают шаблон.
 * Так же было и в старой админке — отбор по папке в TemplateTables.jsx
 * закомментирован.
 */
export function TemplateCreateDialog({
  menuId,
  menuLabel,
  onClose,
}: {
  /** Папка, из которой делают шаблон: её дерево уезжает в шаблон. */
  menuId: string;
  menuLabel: string;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const { current: language } = useDataLanguages();

  const [name, setName] = useState(menuLabel);
  const [description, setDescription] = useState("");
  const [search, setSearch] = useState("");
  const [picked, setPicked] = useState<Set<string>>(new Set());
  /** Один флажок на весь шаблон — смешанный выбор шлюз путает, см. api. */
  const [withRows, setWithRows] = useState(false);

  const tables = useTables(search);
  const create = useCreateTemplate();

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const toggle = (id: string) =>
    setPicked((prev) => {
      const next = new Set(prev);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const ready = name.trim().length > 0 && picked.size > 0;

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (!ready) return;

    create.mutate(
      {
        name: name.trim(),
        description: description.trim(),
        menuId,
        tables: [...picked],
        withRows,
      },
      { onSuccess: onClose },
    );
  };

  return (
    <Modal onClose={onClose}>
      <form
        onSubmit={submit}
        className="flex max-h-[80vh] w-full max-w-md flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
      >
        <h2 className="text-base font-semibold">{t("templateCreate.title")}</h2>

        <Field label={t("templateCreate.name")}>
          <Input
            autoFocus
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </Field>

        <Field label={t("templateCreate.description")}>
          <Input value={description} onChange={(event) => setDescription(event.target.value)} />
        </Field>

        <div className="flex min-h-0 flex-col gap-1.5">
          <span className="flex items-baseline justify-between gap-2">
            <span className="text-xs font-medium text-fg-muted">{t("templateCreate.tables")}</span>
            <span className="text-xs text-fg-subtle">
              {t("templateCreate.picked", { count: picked.size })}
            </span>
          </span>

          <Input
            placeholder={t("templateCreate.search")}
            value={search}
            onChange={(event) => setSearch(event.target.value)}
          />

          <div className="min-h-0 flex-1 overflow-y-auto rounded-md border border-border">
            {tables.items.map((table) => (
              <label
                key={table.id}
                className="flex items-center gap-2 border-b border-border px-2 py-1.5 text-sm last:border-b-0 hover:bg-surface-hover"
              >
                <Checkbox checked={picked.has(table.id)} onChange={() => toggle(table.id)} />
                <span className="truncate">{localized(table.labels, language, table.label)}</span>
              </label>
            ))}

            {!tables.items.length && (
              <p className="p-3 text-sm text-fg-muted">
                {tables.isLoading ? t("common.loading") : t("templateCreate.noTables")}
              </p>
            )}

            {tables.hasMore && (
              <button
                type="button"
                onClick={tables.loadMore}
                className="w-full px-2 py-1.5 text-xs text-fg-muted transition-colors hover:bg-surface-hover"
              >
                {t("action.loadMore")}
              </button>
            )}
          </div>
        </div>

        <label className="flex items-start gap-2 text-sm">
          <Checkbox
            checked={withRows}
            onChange={(event) => setWithRows(event.target.checked)}
            className="mt-0.5"
          />
          <span className="flex flex-col">
            {t("templateCreate.withRows")}
            <span className="text-xs text-fg-subtle">{t("templateCreate.withRowsHint")}</span>
          </span>
        </label>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={create.isPending || !ready}>
            {t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
