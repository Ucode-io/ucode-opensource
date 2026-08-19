import { useState } from "react";
import { IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Popover, PopoverItem } from "@/shared/ui/popover";
import { IMPLEMENTED_VIEW_TYPES, VIEW_TYPES } from "../model/types";
import { viewIcon } from "./view-icon";

/**
 * Новая вкладка: имя и список типов, как в старой админке, — выбор типа
 * и есть создание, отдельной кнопки «создать» нет.
 *
 * Типы — только те, чьи экраны нарисованы: в старой админке список из
 * восьми, но предлагать доску или календарь, за которыми стоит «экран
 * не готов», значит обещать то, чего нет. Список растёт сам — он
 * читается из IMPLEMENTED_VIEW_TYPES.
 *
 * Второго окна с настройками типа (у доски — группирующее поле,
 * у календаря — пара дат) нет тоже: оно нужно типам, которых здесь нет.
 *
 * Имя необязательно: без него вкладка называется своим типом, ровно как
 * все view, созданные до появления имён.
 */
const CREATABLE = VIEW_TYPES.filter((type) => IMPLEMENTED_VIEW_TYPES.has(type));

export function ViewCreateButton({
  busy,
  onCreate,
}: {
  busy: boolean;
  onCreate: (name: string, type: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <Popover
      trigger={({ open, toggle }) => (
        <button
          type="button"
          onClick={toggle}
          disabled={busy}
          aria-expanded={open}
          aria-label={t("view.create")}
          title={t("view.create")}
          className="grid size-7 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg disabled:opacity-50"
        >
          <Icon as={IconPlus} size={16} />
        </button>
      )}
    >
      {(close) => (
        <CreateForm
          onSubmit={(name, type) => {
            onCreate(name, type);
            close();
          }}
        />
      )}
    </Popover>
  );
}

function CreateForm({ onSubmit }: { onSubmit: (name: string, type: string) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");

  const create = (type: string) => onSubmit(name.trim(), type);

  return (
    <form
      className="flex w-56 flex-col gap-1 p-1"
      /* Enter в поле имени — самый частый путь: обычная таблица. */
      onSubmit={(event) => {
        event.preventDefault();
        create(CREATABLE[0] ?? "TABLE");
      }}
    >
      <Input
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t("view.namePlaceholder")}
        aria-label={t("view.name")}
      />

      {CREATABLE.map((type) => (
        <PopoverItem
          key={type}
          icon={<Icon as={viewIcon(type)} size={16} className="shrink-0" />}
          onClick={() => create(type)}
        >
          {t(`view.type.${type}` as TranslationKey, { defaultValue: type })}
        </PopoverItem>
      ))}
    </form>
  );
}
