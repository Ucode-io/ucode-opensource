import { useState } from "react";
import { IconPlus } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Popover } from "@/shared/ui/popover";

/**
 * Новая вкладка.
 *
 * Одно поле — имя. В старой админке на этом месте список из восьми типов,
 * а за ним второе всплывающее окно с настройками того типа, который
 * выбрали: у доски группирующее поле, у календаря пара дат, у сайта
 * ссылка. Здесь рисуется только TABLE, и предлагать остальные значит
 * обещать экран, которого нет.
 *
 * Имя необязательно: без него вкладка называется своим типом, ровно как
 * все view, созданные до появления имён.
 */
export function ViewCreateButton({
  busy,
  onCreate,
}: {
  busy: boolean;
  onCreate: (name: string) => void;
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
          onSubmit={(name) => {
            onCreate(name);
            close();
          }}
        />
      )}
    </Popover>
  );
}

function CreateForm({ onSubmit }: { onSubmit: (name: string) => void }) {
  const { t } = useTranslation();
  const [name, setName] = useState("");

  return (
    <form
      className="flex w-56 flex-col gap-2 p-1"
      onSubmit={(event) => {
        event.preventDefault();
        onSubmit(name.trim());
      }}
    >
      <Input
        autoFocus
        value={name}
        onChange={(event) => setName(event.target.value)}
        placeholder={t("view.namePlaceholder")}
        aria-label={t("view.name")}
      />
      <Button type="submit" size="sm">
        {t("action.create")}
      </Button>
    </form>
  );
}
