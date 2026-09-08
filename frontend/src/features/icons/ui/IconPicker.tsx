import { useState } from "react";
import { IconX } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { DynamicIcon } from "@/shared/ui/dynamic-icon";
import { Icon } from "@/shared/ui/icon";
import { Input } from "@/shared/ui/input";
import { Popover } from "@/shared/ui/popover";
import { useIconSearch } from "../api/search";

/**
 * Выбор иконки. Показывает только коллекцию Tabler — она контурная
 * целиком, поэтому выбранная иконка всегда совпадает по стилю
 * с интерфейсом.
 *
 * Ручной ввод рядом остаётся: он нужен для иконок из нашего CDN
 * и произвольных ссылок, которые пикер не покрывает.
 */
export function IconPicker({
  value,
  type,
  onChange,
}: {
  value: string;
  type: string;
  onChange: (icon: string) => void;
}) {
  const { t } = useTranslation();

  return (
    <div className="flex items-center gap-2">
      <Popover
        trigger={({ open, toggle }) => (
          <button
            type="button"
            onClick={toggle}
            aria-label={t("iconPicker.choose")}
            className={`grid size-9 shrink-0 place-items-center rounded-md border text-fg-muted transition-colors hover:bg-surface-hover ${
              open ? "border-accent text-fg" : "border-border-strong"
            }`}
          >
            <MenuPreview name={value} type={type} />
          </button>
        )}
      >
        {(close) => (
          <IconGrid
            onPick={(icon) => {
              onChange(icon);
              close();
            }}
          />
        )}
      </Popover>

      <Input
        placeholder="tabler:home"
        value={value}
        onChange={(event) => onChange(event.target.value)}
      />

      {value && (
        <button
          type="button"
          onClick={() => onChange("")}
          aria-label={t("iconPicker.clear")}
          className="grid size-9 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconX} />
        </button>
      )}
    </div>
  );
}

/** Превью в кнопке: та же логика, что и в сайдбаре. */
function MenuPreview({ name, type }: { name: string; type: string }) {
  return <DynamicIcon name={name} fallback={<Placeholder type={type} />} />;
}

function Placeholder({ type }: { type: string }) {
  return <span className="text-2xs text-fg-subtle">{type.slice(0, 1) || "?"}</span>;
}

function IconGrid({ onPick }: { onPick: (icon: string) => void }) {
  const { t } = useTranslation();
  const [query, setQuery] = useState("");
  const { data, isLoading, error } = useIconSearch(query);

  return (
    <div className="flex w-72 flex-col gap-2 p-1">
      <Input
        autoFocus
        placeholder={t("iconPicker.search")}
        value={query}
        onChange={(event) => setQuery(event.target.value)}
      />

      {error && <p className="px-1 py-2 text-xs text-danger">{t("iconPicker.failed")}</p>}

      {isLoading ? (
        <div className="grid grid-cols-8 gap-1" aria-hidden>
          {Array.from({ length: 24 }, (_, index) => (
            <div key={index} className="size-8 rounded-md bg-surface-active" />
          ))}
        </div>
      ) : (
        <div className="grid max-h-56 grid-cols-8 gap-1 overflow-y-auto">
          {(data ?? []).map((icon) => (
            <button
              key={icon}
              type="button"
              onClick={() => onPick(icon)}
              title={icon.replace("tabler:", "")}
              className="grid size-8 place-items-center rounded-md text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
            >
              <DynamicIcon name={icon} fallback={null} />
            </button>
          ))}
        </div>
      )}

      {!isLoading && !error && (data ?? []).length === 0 && (
        <p className="px-1 py-2 text-xs text-fg-subtle">{t("iconPicker.empty")}</p>
      )}
    </div>
  );
}
