import { useState, type ComponentProps } from "react";
import { useTranslation } from "react-i18next";
import { Input } from "./input";

/** Поле пароля с глазком. Кнопка не перекрывает текст: поле сужено паддингом. */
export function PasswordInput({ className = "", ...props }: ComponentProps<"input">) {
  const { t } = useTranslation();
  const [visible, setVisible] = useState(false);

  return (
    <div className="relative">
      <Input {...props} type={visible ? "text" : "password"} className={`pr-9 ${className}`} />
      <button
        type="button"
        onClick={() => setVisible((v) => !v)}
        aria-label={t(visible ? "auth.hidePassword" : "auth.showPassword")}
        className="absolute inset-y-0 right-0 grid w-9 place-items-center rounded-r-md text-fg-subtle hover:text-fg"
      >
        {visible ? <EyeOff /> : <Eye />}
      </button>
    </div>
  );
}

const iconProps = {
  width: 16,
  height: 16,
  viewBox: "0 0 16 16",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.3,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
};

const Eye = () => (
  <svg {...iconProps}>
    <path d="M1.5 8s2.4-4 6.5-4 6.5 4 6.5 4-2.4 4-6.5 4S1.5 8 1.5 8Z" />
    <circle cx="8" cy="8" r="1.8" />
  </svg>
);

const EyeOff = () => (
  <svg {...iconProps}>
    <path d="M6.3 3.7A6.9 6.9 0 0 1 8 3.5c4.1 0 6.5 4 6.5 4a12 12 0 0 1-2 2.4M4 4.9A12 12 0 0 0 1.5 7.5s2.4 4 6.5 4c.9 0 1.7-.2 2.4-.5" />
    <path d="M2 2l12 12" />
  </svg>
);
