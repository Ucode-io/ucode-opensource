import { useState } from "react";
import { IconDatabase, IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Checkbox } from "@/shared/ui/checkbox";
import { Icon } from "@/shared/ui/icon";
import { Field, Input } from "@/shared/ui/input";
import { Modal } from "@/shared/ui/modal";
import {
  useConnections,
  useCreateConnection,
  useExternalTables,
  useTrackTables,
  useUntrackTable,
} from "../api/connections";
import { Empty, SectionHeader, Td, Th } from "./parts";

/**
 * Внешние базы: чужой postgres, таблицы которого работают как свои.
 *
 * Два шага и оба обязательны: сначала подключение (строка соединения),
 * потом выбор таблиц. Второй шаг и есть смысл экрана — чужая база
 * обычно вдесятеро больше того, что нужно показать в админке, и
 * «подключить всё» здесь было бы худшим значением по умолчанию.
 *
 * Отмеченная таблица становится обычной [[Table]] проекта: появляется
 * в списке таблиц, ей можно завести пункт меню и view. Снятая исчезает
 * из проекта, но в чужой базе остаётся нетронутой — это не удаление.
 */
export function ConnectionSettings() {
  const { t } = useTranslation();
  const { connections, isLoading } = useConnections();

  const [connectionId, setConnectionId] = useState("");
  const active = connectionId || connections[0]?.id || "";
  const [adding, setAdding] = useState(false);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <SectionHeader title={t("connections.title")} hint={t("connections.hint")}>
        <Button size="sm" onClick={() => setAdding(true)}>
          <Icon as={IconPlus} size={14} />
          {t("connections.create")}
        </Button>
      </SectionHeader>

      <div className="flex min-h-0 flex-1">
        <nav className="flex w-56 shrink-0 flex-col gap-0.5 overflow-y-auto border-r border-border p-2">
          {isLoading && <p className="px-2 py-1 text-xs text-fg-subtle">{t("common.loading")}</p>}
          {!isLoading && !connections.length && (
            <p className="px-2 py-1 text-xs text-fg-subtle">{t("connections.empty")}</p>
          )}

          {connections.map((connection) => (
            <button
              key={connection.id}
              type="button"
              onClick={() => setConnectionId(connection.id)}
              className={`flex h-8 items-center gap-2 rounded-md px-2 text-left text-sm transition-colors ${
                connection.id === active
                  ? "bg-surface-active text-fg"
                  : "text-fg-muted hover:bg-surface-hover hover:text-fg"
              }`}
            >
              <Icon as={IconDatabase} size={14} className="shrink-0" />
              <span className="truncate">{connection.name}</span>
            </button>
          ))}
        </nav>

        {active ? (
          /* key — чтобы отметки не переезжали на соседнее подключение:
             без него это тот же компонент с другими данными. */
          <ExternalTables key={active} connectionId={active} />
        ) : (
          <p className="p-4 text-sm text-fg-subtle">{t("connections.pick")}</p>
        )}
      </div>

      {adding && <ConnectionDialog onClose={() => setAdding(false)} />}
    </div>
  );
}

/**
 * Таблицы подключения.
 *
 * Отметка копится и уезжает одним запросом: ручка принимает список
 * идентификаторов, и по щелчку на каждой строке мы отправляли бы
 * столько запросов, сколько галок поставил человек.
 *
 * Снятие — по одной: другой ручкой, и она принимает ровно одну таблицу.
 */
function ExternalTables({ connectionId }: { connectionId: string }) {
  const { t } = useTranslation();
  const { tables, isLoading } = useExternalTables(connectionId);
  const track = useTrackTables(connectionId);
  const untrack = useUntrackTable(connectionId);

  const [picked, setPicked] = useState<Set<string>>(() => new Set());

  const toggle = (id: string) =>
    setPicked((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <div className="flex h-11 shrink-0 items-center justify-between gap-2 border-b border-border px-3">
        <span className="text-xs text-fg-subtle">
          {t("connections.tablesCount", { count: tables.length })}
        </span>

        {picked.size > 0 && (
          <Button
            size="sm"
            disabled={track.isPending}
            onClick={() =>
              track.mutate([...picked], { onSuccess: () => setPicked(new Set()) })
            }
          >
            {t("connections.trackSelected", { count: picked.size })}
          </Button>
        )}
      </div>

      <div className="min-h-0 flex-1 overflow-auto">
        <table className="w-full border-separate border-spacing-0">
          <thead>
            <tr>
              <Th className="w-10" />
              <Th>{t("connections.table")}</Th>
              <Th className="w-24">{t("connections.fields")}</Th>
              <Th className="w-32" />
            </tr>
          </thead>

          <tbody>
            {isLoading && <Empty text={t("common.loading")} colSpan={4} />}
            {!isLoading && !tables.length && <Empty text={t("connections.noTables")} colSpan={4} />}

            {tables.map((table) => (
              <tr key={table.id} className="hover:bg-surface-hover">
                <Td className="text-center">
                  {/* У отмеченной таблицы флажка нет: она уже своя,
                      и снимают её кнопкой — это другая ручка. */}
                  {!table.tracked && (
                    <Checkbox
                      checked={picked.has(table.id)}
                      onChange={() => toggle(table.id)}
                      aria-label={table.name}
                    />
                  )}
                </Td>

                <Td className="font-mono text-xs">{table.name}</Td>
                <Td className="text-fg-muted">{table.fieldCount || ""}</Td>

                <Td className="text-right">
                  {table.tracked && (
                    <button
                      type="button"
                      disabled={untrack.isPending}
                      onClick={() => untrack.mutate(table.id)}
                      className="h-7 rounded-md px-2 text-xs text-fg-muted transition-colors hover:bg-surface-active hover:text-fg disabled:opacity-50"
                    >
                      {t("connections.untrack")}
                    </button>
                  )}
                </Td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

/**
 * Новое подключение.
 *
 * Строку соединения проверяет сервер: он же по ней читает схему.
 * Своей проверки «а доступна ли база» у браузера нет и быть не может,
 * поэтому форма не изображает её — ошибка приходит из ответа.
 */
function ConnectionDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation();
  const create = useCreateConnection();

  const [name, setName] = useState("");
  const [connectionString, setConnectionString] = useState("");

  return (
    <Modal onClose={onClose}>
      <form
        className="flex w-full max-w-lg flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate({ name, connectionString }, { onSuccess: onClose });
        }}
      >
        <h2 className="text-base font-semibold">{t("connections.create")}</h2>

        <Field label={t("connections.name")}>
          <Input
            autoFocus
            required
            value={name}
            onChange={(event) => setName(event.target.value)}
          />
        </Field>

        <Field label={t("connections.string")} hint={t("connections.stringHint")}>
          <Input
            required
            value={connectionString}
            onChange={(event) => setConnectionString(event.target.value)}
            placeholder="postgres://user:password@host:5432/database"
            className="font-mono text-xs"
          />
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={create.isPending || !name.trim() || !connectionString.trim()}>
            {create.isPending ? t("common.saving") : t("action.create")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}
