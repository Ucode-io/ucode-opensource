import { useEffect, useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { IconPicker } from "@/features/icons";
import { Field, Input } from "@/shared/ui/input";

/**
 * Создание и переименование пункта — одна форма. В старом коде под каждый
 * тип была своя модалка; разница между ними только в заголовке.
 */
export type MenuFormValue = { label: string; icon: string; href: string; slug: string };

export function MenuFormDialog({
  title,
  initial,
  type,
  needsSlug = false,
  busy,
  onSubmit,
  onClose,
}: {
  title: string;
  initial: MenuFormValue;
  type: string;
  /** Только при создании таблицы: слаг задаёт имя таблицы в базе. */
  needsSlug?: boolean;
  busy: boolean;
  onSubmit: (value: MenuFormValue) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [value, setValue] = useState(initial);

  // У ссылки адрес обязателен: пункт без него никуда не ведёт.
  const isLink = type === "LINK";
  const hrefValid = !isLink || isHttpUrl(value.href);
  const slugValid = !needsSlug || SLUG.test(value.slug.trim());

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (value.label.trim() && hrefValid && slugValid) {
      onSubmit({
        ...value,
        label: value.label.trim(),
        href: value.href.trim(),
        slug: value.slug.trim(),
      });
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 grid place-items-center p-4"
      style={{ background: "var(--color-overlay)" }}
      onPointerDown={(event) => event.target === event.currentTarget && onClose()}
    >
      <form
        onSubmit={submit}
        className="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
      >
        <h2 className="text-base font-semibold">{title}</h2>

        <Field label={t("menuForm.label")}>
          {/* autoFocus здесь уместен: диалог открыт ради этого поля. */}
          <Input
            autoFocus
            required
            value={value.label}
            onChange={(event) => setValue((v) => ({ ...v, label: event.target.value }))}
          />
        </Field>

        {needsSlug && (
          <Field
            label={t("menuForm.slug")}
            hint={
              value.slug && !slugValid ? t("menuForm.slugInvalid") : t("menuForm.slugHint")
            }
          >
            <Input
              required
              placeholder="orders"
              value={value.slug}
              onChange={(event) => setValue((v) => ({ ...v, slug: event.target.value }))}
            />
          </Field>
        )}

        {isLink && (
          <Field
            label={t("menuForm.href")}
            hint={
              value.href && !hrefValid ? t("menuForm.hrefInvalid") : t("menuForm.hrefHint")
            }
          >
            <Input
              required
              type="url"
              placeholder="https://docs.u-code.io"
              value={value.href}
              onChange={(event) => setValue((v) => ({ ...v, href: event.target.value }))}
            />
          </Field>
        )}

        <Field label={t("menuForm.icon")} hint={t("menuForm.iconHint")}>
          <IconPicker
            value={value.icon}
            type={type}
            onChange={(icon) => setValue((v) => ({ ...v, icon }))}
          />
        </Field>

        <div className="flex justify-end gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={busy || !value.label.trim() || !hrefValid || !slugValid}
          >
            {t("action.save")}
          </Button>
        </div>
      </form>
    </div>
  );
}

/**
 * Слаг становится именем таблицы в SQL — правит его только человек.
 * Автоподстановка из названия здесь бесполезна: названия у проектов
 * русские, а слаг обязан быть латиницей.
 * ponytail: если понадобится, транслитерация добавляется одним хелпером.
 */
const SLUG = /^[a-z][a-z0-9_]*$/;

/**
 * Разрешаем только http и https. Пункт меню рисуется как <a href={…}>,
 * поэтому адрес вида javascript:... выполнил бы код по клику.
 */
function isHttpUrl(value: string): boolean {
  try {
    const url = new URL(value.trim());
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}
