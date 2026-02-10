import { cn } from "@/lib/utils";

export function PageContent({
	children,
	className,
}: {
	children: React.ReactNode;
	className?: string;
}) {
	return (
		<div className={cn("p-4 md:flex-1 md:overflow-auto md:p-6", className)}>
			{children}
		</div>
	);
}
