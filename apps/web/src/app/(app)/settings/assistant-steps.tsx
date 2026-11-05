import { useTranslations } from "next-intl";

/** Connection steps for each assistant, with a hint to refresh the tool list. */
export function AssistantSteps() {
  const t = useTranslations("settings");
  const steps = [
    { title: t("claudeTitle"), text: t("claudeSteps"), refresh: t("claudeRefresh"), color: "bg-[#ff8f75]" },
    { title: t("chatgptTitle"), text: t("chatgptSteps"), refresh: t("chatgptRefresh"), color: "bg-[#7fb6ff]" },
  ];
  return (
    <ol className="grid gap-2.5">
      {steps.map((step, index) => (
        <li key={step.title} className="flex gap-3 rounded-2xl bg-white/6 p-3.5">
          <span
            className={`flex size-7.5 shrink-0 items-center justify-center rounded-[10px] text-sm font-bold text-[#16133a] ${step.color}`}
          >
            {index + 1}
          </span>
          <div className="grid gap-1">
            <h3 className="font-sans text-[15px] font-bold">{step.title}</h3>
            <p className="text-[13.5px] leading-relaxed text-hero-muted">{step.text}</p>
            <p className="text-[13.5px] leading-relaxed text-hero-muted">{step.refresh}</p>
          </div>
        </li>
      ))}
    </ol>
  );
}
