import { useEffect, useRef, useState } from "react";
import { IconHistory, IconPlus, IconSparkles } from "@tabler/icons-react";
import { useTranslation } from "react-i18next";
import { SidebarToggleButton } from "@/features/sidebar";
import { Icon } from "@/shared/ui/icon";
import { Popover } from "@/shared/ui/popover";
import { useCopilotChat } from "../model/use-chat";
import { Composer, Greeting, HeaderButton, History } from "./CopilotPanel";
import { Conversation } from "./Conversation";

/**
 * Помощник во весь экран. Те же беседа, шаги и подсказки, что в правой
 * панели, — но с местом под длинный ответ: схему проекта в панели
 * шириной 380px читают по три слова в строке.
 *
 * Колонка держится 768px и стоит по центру, как в любой переписке:
 * строка во всю ширину монитора не читается, а поле ввода, разъехавшееся
 * на 2000px, перестаёт быть полем ввода.
 *
 * ponytail: беседа у страницы своя, отдельная от панели. Общее состояние —
 * если начнут путать: история бесед и так одна, она на сервере.
 */
export function CopilotPage() {
  const { t } = useTranslation();
  const chat = useCopilotChat();
  const box = useRef<HTMLTextAreaElement>(null);
  const [input, setInput] = useState("");

  // Страницу открывают, чтобы что-то спросить: курсор ставится сам.
  useEffect(() => box.current?.focus(), []);

  const submit = (text: string) => {
    if (!text.trim() || chat.sending) return;

    void chat.send(text);
    setInput("");
  };

  const fill = (prompt: string) => {
    setInput(prompt);
    box.current?.focus();
  };

  return (
    <div className="animate-page flex h-full flex-col">
      {/* Шапка — та же, что у пункта меню: кнопка сайдбара, имя экрана,
          действия справа. */}
      <header className="flex h-header shrink-0 items-center gap-2 border-b border-border px-4">
        <SidebarToggleButton />
        <span className="grid size-6 shrink-0 place-items-center rounded-md bg-accent-subtle text-accent-text">
          <Icon as={IconSparkles} size={14} />
        </span>
        <span className="truncate text-sm font-medium" title={chat.title}>
          {chat.title ? firstLine(chat.title) : t("copilot.title")}
        </span>

        <div className="ml-auto flex items-center gap-0.5">
          <Popover
            align="end"
            trigger={({ toggle }) => (
              <HeaderButton icon={IconHistory} label={t("copilot.history")} onClick={toggle} />
            )}
          >
            {(close) => <History onPick={chat.open} close={close} />}
          </Popover>

          <HeaderButton
            icon={IconPlus}
            label={t("copilot.newChat")}
            onClick={() => {
              chat.reset();
              setInput("");
              box.current?.focus();
            }}
          />
        </div>
      </header>

      {/* min-h-0 — иначе колонка растягивается по содержимому, и
          прокручивается страница целиком вместе с полем ввода. */}
      <div className="flex min-h-0 flex-1 justify-center">
        <div className="flex min-h-0 w-full max-w-3xl flex-col">
          {chat.messages.length > 0 || chat.run ? (
            <Conversation messages={chat.messages} run={chat.run} />
          ) : (
            <Greeting onPick={fill} />
          )}

          <Composer
            inputRef={box}
            value={input}
            onChange={setInput}
            onSubmit={submit}
            onStop={chat.stop}
            busy={chat.sending}
          />

          {/* Оговорка обязательна: помощник правит СХЕМУ проекта, а не
              отвечает словами. Ошибку в ответе видно сразу, ошибку
              в созданной таблице — через неделю. */}
          <p className="shrink-0 pb-3 text-center text-2xs text-fg-subtle">
            {t("copilot.disclaimer")}
          </p>
        </div>
      </div>
    </div>
  );
}

/** Первая строка заголовка беседы: остальное в шапку всё равно не влезет. */
const firstLine = (title: string) => title.split("\n")[0]?.trim() ?? "";
