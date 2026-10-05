"use client";

import { CheckIcon, CopyIcon } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState } from "react";

import { Button } from "@/components/ui/button";

export function CopyButton({ value, className }: { value: string; className?: string }) {
  const t = useTranslations("common");
  const [copied, setCopied] = useState(false);

  async function copy() {
    await navigator.clipboard.writeText(value);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  }

  return (
    <Button type="button" variant="outline" size="sm" onClick={copy} className={className}>
      {copied ? <CheckIcon /> : <CopyIcon />}
      {copied ? t("copied") : t("copy")}
    </Button>
  );
}
