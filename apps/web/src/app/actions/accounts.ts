"use server";

import { revalidatePath } from "next/cache";
import { getTranslations } from "next-intl/server";

import { failure, success } from "@/lib/action-errors";
import { ApiError, unwrap } from "@/lib/api/client";
import { parseSignedAmount, text, type FormState } from "@/lib/forms";
import { authedApi } from "@/lib/session";

type AccountType = "checking" | "savings" | "credit_card" | "cash" | "investment" | "other";

export async function saveAccount(_: FormState, formData: FormData): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  const id = text(formData, "id");
  const name = text(formData, "name");
  const type = text(formData, "type") as AccountType;

  try {
    // The opening balance is entered as a decimal in the account currency.
    let minorUnits: number;
    if (id) {
      minorUnits = unwrap(await api.GET("/api/v1/accounts/{id}", { params: { path: { id } } })).minor_units;
    } else {
      const currencies = unwrap(await api.GET("/api/v1/currencies")).items;
      const currency = currencies.find((c) => c.code === text(formData, "currency"));
      if (!currency) {
        return { ok: false, message: t("errors.validation"), fieldErrors: { currency: t("errors.required") } };
      }
      minorUnits = currency.minor_units;
    }

    const fieldErrors: Record<string, string> = {};
    if (!name) {
      fieldErrors.name = t("errors.required");
    }
    const initialBalance = parseSignedAmount(text(formData, "initial_balance"), minorUnits);
    if (initialBalance === null) {
      fieldErrors.initial_balance = t("errors.invalidInitialBalance", { decimals: minorUnits });
    }
    if (Object.keys(fieldErrors).length > 0) {
      return { ok: false, message: t("errors.validation"), fieldErrors };
    }

    if (id) {
      unwrap(
        await api.PATCH("/api/v1/accounts/{id}", {
          params: { path: { id } },
          body: { name, type, initial_balance: initialBalance ?? 0 },
        }),
      );
    } else {
      unwrap(
        await api.POST("/api/v1/accounts", {
          body: { name, type, currency: text(formData, "currency"), initial_balance: initialBalance ?? 0 },
        }),
      );
    }
  } catch (error) {
    return failure(error, { conflict: t("accounts.nameTaken") });
  }
  revalidatePath("/", "layout");
  return success();
}

export async function setAccountArchived(id: string, archived: boolean): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  try {
    unwrap(await api.PATCH("/api/v1/accounts/{id}", { params: { path: { id } }, body: { archived } }));
  } catch (error) {
    return failure(error, { conflict: t("accounts.nameTaken") });
  }
  revalidatePath("/", "layout");
  return success();
}

export async function deleteAccount(id: string): Promise<FormState> {
  const t = await getTranslations();
  const api = await authedApi();
  const result = await api.DELETE("/api/v1/accounts/{id}", { params: { path: { id } } });
  if (!result.response.ok) {
    return failure(new ApiError(result.response.status, result.error), {
      conflict: t("accounts.deleteConflict"),
    });
  }
  revalidatePath("/", "layout");
  return success();
}
