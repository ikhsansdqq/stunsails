import { cva, type VariantProps } from "class-variance-authority";
import type { ButtonHTMLAttributes } from "react";
import type { LucideIcon } from "@/components/ui/lucide";
import { cn } from "@/lib/utils";

// Variants follow ems-dashboard's shadcn Button; the shape follows the sign-in page:
// pills, and a primary that slides an arrow in on hover (.btn-arrow in style.css).
const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-2 rounded-full text-sm font-medium whitespace-nowrap transition-[color,background-color,border-color,box-shadow,opacity,scale,padding] duration-200 ease-[cubic-bezier(0.22,1,0.36,1)] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
  {
    variants: {
      variant: {
        default: "btn-arrow bg-primary text-primary-foreground hover:bg-primary-hover focus-visible:bg-primary-hover",
        destructive: "border border-critical/30 bg-card text-critical hover:bg-critical/8",
        outline: "border bg-card shadow-xs hover:bg-accent hover:text-primary dark:border-input",
        ghost: "text-muted-foreground hover:bg-accent hover:text-primary",
      },
      size: {
        default: "h-9 px-5 [--btn-gap:0.5rem] [--btn-pad:1.25rem] has-[>svg,>.btn-icon]:px-4",
        sm: "h-8 gap-1.5 px-4 [--btn-gap:0.375rem] [--btn-pad:1rem] has-[>svg,>.btn-icon]:px-3",
        icon: "size-9",
        "icon-sm": "size-8",
      },
      // Pressed buttons shrink a touch; static opts out where the motion would distract.
      press: {
        scale: "motion-safe:not-disabled:active:scale-[0.96]",
        static: "",
      },
    },
    defaultVariants: { variant: "default", size: "default", press: "scale" },
  },
);

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, VariantProps<typeof buttonVariants> {
  /** A leading icon. */
  icon?: LucideIcon;
}

export function Button({ className, variant, size, press, type = "button", icon: Icon, children, ...props }: ButtonProps) {
  return (
    <button type={type} className={cn(buttonVariants({ variant, size, press }), className)} {...props}>
      {Icon && <Icon size={16} className="btn-icon" aria-hidden />}
      {children}
    </button>
  );
}

export { buttonVariants };
