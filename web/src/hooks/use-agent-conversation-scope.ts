import { useCallback, useLayoutEffect, useRef } from "react";
import { getActiveUserScope } from "@/lib/user-scope";

/** A late async continuation belongs to its original account and mounted conversation. */
export function useAgentConversationScope(conversationId: string) {
    const account = useRef(getActiveUserScope());
    const currentScope = useRef(conversationId);
    currentScope.current = conversationId;
    useLayoutEffect(() => {
        currentScope.current = conversationId;
        return () => {
            currentScope.current = "";
        };
    }, [conversationId]);
    const isCurrentScope = useCallback((scope: string) => Boolean(scope) && currentScope.current === scope && getActiveUserScope() === account.current, []);
    return { currentScope, isCurrentScope };
}
