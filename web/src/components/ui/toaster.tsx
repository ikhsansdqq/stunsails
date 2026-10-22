import type { CSSProperties } from "react";
import { Toaster as Sonner } from "sonner";
import { CircleAlert, CircleCheck, Info, LoaderCircle, TriangleAlert, X } from "@/components/ui/lucide";
import { useTheme } from "@/hooks/use-theme";

const icons = {
  success: <CircleCheck size={16} aria-hidden />,
  info: <Info size={16} aria-hidden />,
  warning: <TriangleAlert size={16} aria-hidden />,
  error: <CircleAlert size={16} aria-hidden />,
  loading: <LoaderCircle size={16} className="animate-spin" aria-hidden />,
  close: <X size={12} aria-hidden />,
};

/**
 * Where every toast appears: bottom-right on a desktop, full width at the bottom
 * on a phone (Sonner switches below 600px), in the app's colours and theme.
 * Above dialogs, so a save inside one still confirms.
 */
export function Toaster() {
  const { resolved } = useTheme();
  return (
    <Sonner
      theme={resolved}
      position="bottom-right"
      closeButton
      icons={icons}
      visibleToasts={4}
      offset={16}
      mobileOffset={{ bottom: "calc(12px + env(safe-area-inset-bottom))", left: 12, right: 12 }}
      style={
        {
          "--normal-bg": "var(--card)",
          "--normal-text": "var(--foreground)",
          "--normal-border": "var(--border)",
          "--success-bg": "var(--card)",
          "--success-text": "var(--foreground)",
          "--success-border": "var(--border)",
          "--error-bg": "var(--card)",
          "--error-text": "var(--foreground)",
          "--error-border": "color-mix(in oklch, var(--critical) 40%, var(--border))",
          "--border-radius": "var(--radius)",
          zIndex: 100,
        } as CSSProperties
      }
      toastOptions={{
        classNames: {
          toast: "font-sans shadow-lg",
          title: "text-sm font-medium",
          description: "text-[0.8125rem] !text-muted-foreground",
          success: "[&_[data-icon]]:text-good",
          error: "[&_[data-icon]]:text-critical",
          warning: "[&_[data-icon]]:text-warning",
          info: "[&_[data-icon]]:text-primary",
          actionButton: "!bg-primary !text-primary-foreground !rounded-md !font-medium",
        },
      }}
    />
  );
}
