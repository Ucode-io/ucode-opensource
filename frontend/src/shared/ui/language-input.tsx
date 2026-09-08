import { useState } from "react";
import { IconLanguage } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { CommitInput } from "./commit-input";
import { Icon } from "./icon";
import { Input } from "./input";

/**
 * Имя на нескольких языках — ОДНО поле ввода с переключателем языка
 * внутри, а не по полю на язык.
 *
 * Так это сделано в старой админке (NewFormElements/
 * TextFieldWithMultiLanguage): кнопка справа показывает текущий язык
 * и по щелчку переходит к следующему. Столбик из полей выглядит формой
 * на пять обязательных строк, хотя заполняют почти всегда одно: языков
 * у проекта бывает четыре, а полей в диалоге — три.
 *
 * Переключение по кругу, а не списком: с двумя языками список — это
 * лишний щелчок, а больше двух в проектах почти не встречается.
 *
 * Один язык — кнопки нет вовсе: переключать нечего, и обычное поле
 * ввода честнее.
 */
export type InputLanguage = {
  code: string;
  /** Название языка — только для подписи кнопки читалке экрана. */
  nativeName: string;
};

type Common = {
  languages: InputLanguage[];
  /** Значения по коду языка. Отсутствующий ключ — пустое поле. */
  values: Record<string, string>;
  /** Подпись для читалки экрана: видимой подписи у поля обычно нет. */
  label: string;
  placeholder?: string;
  autoFocus?: boolean;
};

/**
 * Две повадки поля, и выбирает её вызывающий:
 *
 *   onChange — значение нужно на каждую букву (диалог со своей кнопкой
 *              «Сохранить», где всё уезжает разом);
 *   onCommit — значение уезжает запросом, и слать его на каждую букву
 *              нельзя (см. CommitInput).
 */
type Live = {
  onChange: (code: string, value: string) => void;
  onCommit?: never;
  /** Enter в поле: у диалога это «готово», у панели поля — «создать». */
  onEnter?: () => void;
};
type Committed = {
  onCommit: (code: string, value: string) => void;
  onChange?: never;
  /** Пустая строка — тоже значение, и её нужно отправить. */
  allowEmpty?: boolean;
};

export function LanguageInput({
  languages,
  values,
  label,
  placeholder,
  autoFocus,
  ...mode
}: Common & (Live | Committed)) {
  const { t } = useTranslation();
  const [at, setAt] = useState(0);

  const current = languages[at] ?? languages[0];
  if (!current) return null;

  const value = values[current.code] ?? "";
  /* Место под кнопку: без отступа текст уезжает под неё. */
  const room = languages.length > 1 ? "pr-16" : "";

  return (
    <span className="relative flex w-full items-center">
      {mode.onCommit ? (
        <CommitInput
          /*
           * Ключ по языку: CommitInput держит набранное в своём
           * состоянии и не следит за пропом. Без ключа переключение
           * языка оставляло бы в поле текст предыдущего.
           */
          key={current.code}
          value={value}
          label={label}
          className={room}
          allowEmpty={mode.allowEmpty ?? false}
          onCommit={(next) => mode.onCommit(current.code, next)}
          {...(placeholder === undefined ? {} : { placeholder })}
          {...(autoFocus ? { autoFocus: true } : {})}
        />
      ) : (
        <Input
          value={value}
          aria-label={label}
          className={room}
          onChange={(event) => mode.onChange(current.code, event.target.value)}
          onKeyDown={(event) => {
            if (event.key !== "Enter" || !mode.onEnter) return;
            event.preventDefault();
            mode.onEnter();
          }}
          {...(placeholder === undefined ? {} : { placeholder })}
          {...(autoFocus ? { autoFocus: true } : {})}
        />
      )}

      {languages.length > 1 && (
        <button
          type="button"
          onClick={() => setAt((index) => (index + 1) % languages.length)}
          aria-label={t("common.inputLanguage", { language: current.nativeName })}
          title={t("common.inputLanguage", { language: current.nativeName })}
          className="absolute right-1 flex h-6 shrink-0 items-center gap-1 rounded px-1.5 text-2xs text-fg-muted uppercase transition-colors hover:bg-surface-hover hover:text-fg"
        >
          <Icon as={IconLanguage} size={14} />
          {current.code}
        </button>
      )}
    </span>
  );
}
