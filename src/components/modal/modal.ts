import SuperComponent from "@codewithkyle/supercomponent";
import { html, render, TemplateResult } from "lit-html";
import env from "~brixi/controllers/env";

interface IModal {
    view: TemplateResult | string,
    heading: string,
    message: string,
    size: "small" | "medium" | "large" | string;
}
export default class Modal extends SuperComponent<IModal>{
    constructor(size: "small" | "medium" | "large" | string = "small"){
        super();
        this.model = {
            view: "",
            heading: null,
            message: null,
            size: size,
        };
    }

    override connected(): void {
        this.render();
    }

    public setHeading(heading:string):void{
        if (this.isConnected){
            this.set({
                heading: heading,
            });
        } else  {
            this.model.heading = heading;
        }
    }

    public setMessage(message:string):void{
        if (this.isConnected){
            this.set({
                message: message,
            });
        } else {
            this.model.message = message;
        }
    }

    public setView(view:TemplateResult | string):void {
        if (this.isConnected){
            this.set({
                view: view,
            });
        } else {
            this.model.view = view;
        }
    }

    public close:EventListener = (e:Event) => {
        this.remove();
    }

    override render(): void {
        const view = html`
            <div @click=${this.close} class="backdrop"></div>
			<div class="modal" size="${this.model.size}">
				${ this.model.heading?.length ? html`<h1>${this.model.heading}</h1>` : "" }
				${ this.model.message?.length ? html`<p>${this.model.message}</p>` : "" }
				<button class="close bttn" kind="text" color="grey" icon="center" shape="round" aria-label="close modal" @click=${this.close}>
                    <svg xmlns="http://www.w3.org/2000/svg" class="icon icon-tabler icon-tabler-x" width="24" height="24" viewBox="0 0 24 24" stroke-width="2" stroke="currentColor" fill="none" stroke-linecap="round" stroke-linejoin="round">
                        <path stroke="none" d="M0 0h24v24H0z" fill="none"></path>
                        <line x1="18" y1="6" x2="6" y2="18"></line>
                        <line x1="6" y1="6" x2="18" y2="18"></line>
                    </svg>
				</button>
                ${this.model.view}
			</div>
        `;
        render(view, this);
    }
}
env.bind("modal-component", Modal);
