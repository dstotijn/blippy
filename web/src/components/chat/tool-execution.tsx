import { Check, ChevronDown, Copy, Terminal } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
	Collapsible,
	CollapsibleContent,
	CollapsibleTrigger,
} from "@/components/ui/collapsible";
import { cn } from "@/lib/utils";

interface ToolExecutionProps {
	name: string;
	input?: string;
	result?: string;
}

export function ToolExecution({ name, input, result }: ToolExecutionProps) {
	const [isOpen, setIsOpen] = useState(true);
	const [copied, setCopied] = useState(false);

	const formatInput = (input: string) => {
		try {
			const parsed = JSON.parse(input);
			if (parsed.command) return `$ ${parsed.command}`;
			return JSON.stringify(parsed, null, 2);
		} catch {
			return input;
		}
	};

	const copyToClipboard = async () => {
		const text = result || input || "";
		await navigator.clipboard.writeText(text);
		setCopied(true);
		setTimeout(() => setCopied(false), 2000);
	};

	return (
		<Collapsible open={isOpen} onOpenChange={setIsOpen}>
			<div className="rounded-lg border bg-muted/50">
				<CollapsibleTrigger asChild>
					<button
						type="button"
						className="flex w-full items-center justify-between p-3 text-left"
					>
						<div className="flex items-center gap-2 text-muted-foreground">
							<Terminal className="h-4 w-4" />
							<span className="font-medium">{name}</span>
						</div>
						<ChevronDown
							className={cn(
								"h-4 w-4 text-muted-foreground transition-transform",
								isOpen && "rotate-180",
							)}
						/>
					</button>
				</CollapsibleTrigger>
				<CollapsibleContent>
					<div className="border-t px-3 pb-3">
						{input && (
							<pre className="mt-2 overflow-x-auto whitespace-pre-wrap break-all font-mono text-sm">
								{formatInput(input)}
							</pre>
						)}
						{result && (
							<div className="relative mt-2 border-t pt-2">
								<Button
									variant="ghost"
									size="icon"
									className="absolute right-0 top-2 h-6 w-6"
									onClick={copyToClipboard}
								>
									{copied ? (
										<Check className="h-3 w-3" />
									) : (
										<Copy className="h-3 w-3" />
									)}
								</Button>
								<pre className="max-h-48 overflow-auto whitespace-pre-wrap break-all pr-8 font-mono text-sm text-muted-foreground">
									{result}
								</pre>
							</div>
						)}
					</div>
				</CollapsibleContent>
			</div>
		</Collapsible>
	);
}
