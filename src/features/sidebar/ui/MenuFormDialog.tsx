import { useEffect, useState, type FormEvent } from "react";
import { IconTrash } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { IconPicker } from "@/features/icons";
import { useMicrofrontends } from "@/features/microfrontend";
import { Checkbox } from "@/shared/ui/checkbox";
import { useDataLanguages } from "@/features/workspace";
import { Button } from "@/shared/ui/button";
import { Icon } from "@/shared/ui/icon";
import type { TranslationKey } from "@/shared/lib/i18n";
import { Field, Input, Select } from "@/shared/ui/input";
import { slugify } from "@/shared/lib/slug";
import { LanguageInput } from "@/shared/ui/language-input";
import { Modal } from "@/shared/ui/modal";

/**
 * Создание и переименование пункта — одна форма. В старом коде под каждый
 * тип была своя модалка; разница между ними только в заголовке.
 */
export type MenuFormValue = {
  /** Подписи по языкам ДАННЫХ проекта: ключ — код языка. */
  labels: Record<string, string>;
  icon: string;
  href: string;
  slug: string;
  /** Папка в файловом хранилище — только у пункта MINIO_FOLDER. */
  folder: string;
  /**
   * Открывать адрес ВНУТРИ админки, рамкой, а не новой вкладкой.
   * Это разные настройки пункта: `website_link` против `link`.
   */
  embed: boolean;
  /** Какое чужое приложение показывает пункт — только у MICROFRONTEND. */
  microfrontendId: string;
  /** Настройки запуска ремоута: пары «ключ — значение», порядок их. */
  params: { key: string; value: string }[];
};

/** Типы пунктов, которые заводят из сайдбара. */
export type CreatableType = "FOLDER" | "TABLE" | "LINK" | "MINIO_FOLDER" | "MICROFRONTEND";

/** Заголовок окна создания зависит только от типа. */
export const CREATE_TITLES: Record<CreatableType, TranslationKey> = {
  FOLDER: "menuForm.createFolder",
  TABLE: "menuForm.createTable",
  LINK: "menuForm.createLink",
  MINIO_FOLDER: "menuForm.createFiles",
  MICROFRONTEND: "menuForm.createMicrofrontend",
};

/** Пустое значение формы. Одно на все места, где заводят пункт. */
export const EMPTY_MENU_FORM: MenuFormValue = {
  labels: {},
  icon: "",
  href: "",
  slug: "",
  folder: "",
  embed: false,
  microfrontendId: "",
  params: [],
};

/**
 * Что из формы уезжает в `attributes` — по типу пункта. Одно место
 * на создание и на правку: раньше эта развилка была написана дважды,
 * в кнопке «+» и в меню строки, и новый тип пришлось бы дописывать
 * в обе.
 */
export function menuAttributes(type: string, value: MenuFormValue): Record<string, unknown> {
  if (type === "LINK") {
    return value.embed ? { website_link: value.href } : { link: value.href };
  }
  if (type === "MINIO_FOLDER") return { path: value.folder };
  if (type === "MICROFRONTEND") {
    // Пустые строки не сохраняем: пара без ключа ремоуту не нужна,
    // а в форме она остаётся от нажатой и незаполненной кнопки «+».
    return { params: value.params.filter((param) => param.key.trim()) };
  }
  return {};
}

export function MenuFormDialog({
  title,
  initial,
  type,
  needsSlug = false,
  needsRemote = false,
  busy,
  onSubmit,
  onClose,
}: {
  title: string;
  initial: MenuFormValue;
  type: string;
  /** Только при создании таблицы: слаг задаёт имя таблицы в базе. */
  needsSlug?: boolean;
  /** Только при создании микрофронтенда: выбрать, какое приложение. */
  needsRemote?: boolean;
  busy: boolean;
  onSubmit: (value: MenuFormValue) => void;
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const [value, setValue] = useState(initial);
  /**
   * Слаг, тронутый руками, из названия больше не переписывается.
   * У пункта, который открыли править, он тронут с самого начала:
   * имя таблицы в базе задаётся один раз.
   */
  const [slugTouched, setSlugTouched] = useState(Boolean(initial.slug));
  /*
   * Языки ДАННЫХ проекта, а не локали интерфейса: подпись пункта живёт
   * в attributes.label_<код языка проекта>, теми же ключами, что подписи
   * полей и имена view.
   */
  const { languages } = useDataLanguages();

  // У ссылки адрес обязателен: пункт без него никуда не ведёт.
  const isLink = type === "LINK";
  /*
   * У хранилища обязательна папка: пустая означала бы «все файлы
   * проекта», а пункт заводят под конкретную папку. Правила те же,
   * что у слага: латиница, цифры и подчёркивание.
   */
  const isFiles = type === "MINIO_FOLDER";
  /*
   * Микрофронтенд выбирают ТОЛЬКО при создании: PUT /v3/menus
   * колонку `microfrontend_id` не пишет вовсе (menu.go:944 — её нет
   * в SET), и список в форме правки был бы переключателем, который
   * ничего не переключает. См. docs/backend-notes.md, «Меню».
   */
  const picksRemote = type === "MICROFRONTEND" && needsRemote;
  const folderValid = !isFiles || SLUG.test(value.folder.trim());
  const hrefValid = !isLink || isHttpUrl(value.href);
  const slugValid = !needsSlug || SLUG.test(value.slug.trim());
  const remoteValid = !picksRemote || Boolean(value.microfrontendId);
  // Хотя бы одно имя: пункт без единой подписи в сайдбаре — пустая строка.
  const named = Object.values(value.labels).some((label) => label.trim());

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => event.key === "Escape" && onClose();
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, [onClose]);

  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (named && hrefValid && slugValid && folderValid && remoteValid) {
      onSubmit({
        ...value,
        href: value.href.trim(),
        slug: value.slug.trim(),
        folder: value.folder.trim(),
      });
    }
  };

  return (
    <Modal onClose={onClose}>
      <form
        onSubmit={submit}
        className="flex w-full max-w-sm flex-col gap-4 rounded-xl border border-border bg-surface p-5 shadow-modal"
      >
        <h2 className="text-base font-semibold">{title}</h2>

        {/* Имя на каждом языке данных — одним полем с переключателем:
            заданное только на русском оставляет узбекский сайдбар
            со слагом, а столбик из полей на четыре языка выглядит
            формой, которую обязаны заполнить целиком. */}
        <div className="flex flex-col gap-1.5">
          <span className="text-xs font-medium text-fg-muted">{t("menuForm.label")}</span>

          <LanguageInput
            autoFocus
            languages={languages}
            values={value.labels}
            label={t("menuForm.label")}
            onChange={(code, label) =>
              setValue((v) => ({
                ...v,
                labels: { ...v.labels, [code]: label },
                /*
                 * Слаг подставляется из названия — с транслитерацией:
                 * названия в проектах русские и узбекские, а слаг
                 * становится ИМЕНЕМ ТАБЛИЦЫ в SQL. Раньше это поле
                 * оставалось пустым, и латиницу набирали руками
                 * на каждой таблице.
                 *
                 * Берётся первый язык, на котором что-то написано:
                 * набирают обычно один, а какой именно — дело проекта.
                 */
                ...(needsSlug && !slugTouched
                  ? { slug: slugify(firstLabel({ ...v.labels, [code]: label })) }
                  : {}),
              }))
            }
          />
        </div>

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
              onChange={(event) => {
                setSlugTouched(true);
                setValue((v) => ({ ...v, slug: event.target.value }));
              }}
            />
          </Field>
        )}

        {isFiles && (
          <Field
            label={t("menuForm.folder")}
            hint={
              value.folder && !folderValid
                ? t("menuForm.slugInvalid")
                : t("menuForm.folderHint")
            }
          >
            <Input
              required
              placeholder="media"
              value={value.folder}
              onChange={(event) => setValue((v) => ({ ...v, folder: event.target.value }))}
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

            {/* Две разные настройки пункта, а не одна с флажком у нас
                в голове: снаружи это `link`, внутри — `website_link`,
                и старая админка их тоже различала. */}
            <label className="mt-2 flex items-center gap-2 text-xs text-fg-muted">
              <Checkbox
                checked={value.embed}
                onChange={(event) => setValue((v) => ({ ...v, embed: event.target.checked }))}
              />
              {t("menuForm.embed")}
            </label>
          </Field>
        )}

        {type === "MICROFRONTEND" && (
          <RemoteFields
            value={value}
            picksRemote={picksRemote}
            onChange={(next) => setValue((v) => ({ ...v, ...next }))}
          />
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
            disabled={busy || !named || !hrefValid || !slugValid || !remoteValid}
          >
            {t("action.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/**
 * Настройки пункта-микрофронтенда: какое приложение и с какими
 * параметрами.
 *
 * Параметры — список пар, как в старой админке
 * (`MicrofrontendLinkModal.jsx:190`). Порядок сохраняем: он ничего
 * не значит для ремоута, но человек их так расставил.
 */
function RemoteFields({
  value,
  picksRemote,
  onChange,
}: {
  value: MenuFormValue;
  picksRemote: boolean;
  onChange: (next: Partial<MenuFormValue>) => void;
}) {
  const { t } = useTranslation();
  const { items, isLoading } = useMicrofrontends(picksRemote);

  const patchParam = (index: number, next: Partial<{ key: string; value: string }>) =>
    onChange({
      params: value.params.map((param, at) => (at === index ? { ...param, ...next } : param)),
    });

  return (
    <>
      {picksRemote && (
        <Field
          label={t("menuForm.microfrontend")}
          hint={isLoading ? t("common.loading") : t("menuForm.microfrontendHint")}
        >
          <Select
            required
            value={value.microfrontendId}
            onChange={(event) => onChange({ microfrontendId: event.target.value })}
          >
            <option value="">{t("menuForm.microfrontendPick")}</option>
            {items.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name}
              </option>
            ))}
          </Select>
        </Field>
      )}

      <Field label={t("menuForm.params")} hint={t("menuForm.paramsHint")}>
        <div className="flex flex-col gap-1.5">
          {value.params.map((param, index) => (
            <div key={index} className="flex items-center gap-1.5">
              <Input
                placeholder={t("menuForm.paramKey")}
                value={param.key}
                onChange={(event) => patchParam(index, { key: event.target.value })}
              />
              <Input
                placeholder={t("menuForm.paramValue")}
                value={param.value}
                onChange={(event) => patchParam(index, { value: event.target.value })}
              />
              <button
                type="button"
                aria-label={t("action.delete")}
                onClick={() =>
                  onChange({ params: value.params.filter((_, at) => at !== index) })
                }
                className="grid size-8 shrink-0 place-items-center rounded-md text-fg-subtle transition-colors hover:bg-surface-hover hover:text-danger"
              >
                <Icon as={IconTrash} size={14} />
              </button>
            </div>
          ))}

          <button
            type="button"
            onClick={() => onChange({ params: [...value.params, { key: "", value: "" }] })}
            className="h-8 rounded-md border border-dashed border-border-strong text-xs text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg"
          >
            {t("menuForm.paramAdd")}
          </button>
        </div>
      </Field>
    </>
  );
}

/**
 * Слаг становится именем таблицы в SQL, поэтому проверяется тем же
 * правилом, что и слаг поля: латиница, цифры и подчёркивание, первая
 * буква не цифра.
 */
const SLUG = /^[a-z][a-z0-9_]*$/;

/** Название на первом заполненном языке: из него и получается слаг. */
function firstLabel(labels: Record<string, string>): string {
  return Object.values(labels).find((label) => label.trim()) ?? "";
}

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
