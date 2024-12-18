import { TemplateResult } from "lit-html";
import "~brixi/components/buttons/button/button";
import "~brixi/components/buttons/submit-button/submit-button";
import "~brixi/components/form/form";
interface DangerousSettings {
    title: string;
    message: string;
    confirm?: string;
    cancel?: string;
    width?: number;
    callbacks?: {
        cancel?: () => void;
        confirm?: () => void;
    };
}
interface ConfirmSettings {
    title: string;
    message: string;
    confirm?: string;
    cancel?: string;
    width?: number;
    callbacks?: {
        cancel?: () => void;
        confirm?: () => void;
    };
}
interface PassiveSettings {
    title: string;
    message: string;
    width?: number;
    actions?: Array<{
        label: string;
        callback: () => void;
    }>;
}
interface FormSettings {
    title?: string;
    message?: string;
    width?: number;
    view: TemplateResult;
    callbacks?: {
        submit?: (data: {
            [key: string]: any;
        }, form: HTMLElement, modal: HTMLElement) => void;
        cancel?: () => void;
    };
    cancel?: string;
    submit?: string;
}
interface RawSettings {
    view: TemplateResult | HTMLElement;
    width?: number;
}
declare class ModalMaker {
    raw(settings: RawSettings): ModalComponent;
    form(settings: FormSettings): void;
    passive(settings: PassiveSettings): void;
    confirm(settings: ConfirmSettings): void;
    dangerous(settings: DangerousSettings): void;
}
declare const modals: ModalMaker;
export default modals;
declare class ModalComponent extends HTMLElement {
    private view;
    private width;
    constructor(view: TemplateResult | HTMLElement, width: number, className: string);
    private render;
}
