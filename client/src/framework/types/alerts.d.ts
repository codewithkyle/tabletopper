import type { NotificationButton } from "@codewithkyle/notifyjs";
declare class Alerts {
    snackbar(message: string, buttons?: Array<NotificationButton>): void;
    /**
     * Notify a user that something has happened.
     */
    alert(title: string, message: string, actions?: Array<{
        label: string;
        callback: Function;
    }>, duration?: number): void;
    /**
     * Notify a user that an action they triggered has succeeded.
     */
    success(title: string, message: string, actions?: Array<{
        label: string;
        callback: Function;
    }>, duration?: number): void;
    /**
     * Warn the user of something.
     */
    warn(title: string, message: string, actions?: Array<{
        label: string;
        callback: Function;
    }>, duration?: number): void;
    /**
     * Notify the user that an action they triggered has failed.
     */
    error(title: string, message: string, actions?: Array<{
        label: string;
        callback: Function;
    }>, duration?: number): void;
    /**
     * Add a custom toast element to the toaster.
     */
    append(toast: HTMLElement): void;
    toast(message: string, duration?: number): void;
}
declare const alerts: Alerts;
export default alerts;
