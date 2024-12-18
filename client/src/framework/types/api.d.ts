export interface Request {
    route: string;
    method?: Method;
    origin?: string;
    body?: BodyParams;
    headers?: Headers;
    params?: GetParams;
    output?: "JSON" | "Blob" | "Text";
}
export interface Response {
    title: string | null;
    message: string | null;
    status: number;
    code: string;
    data: any;
    success: boolean;
}
export type Headers = {
    [header: string]: string;
};
export type GetParams = {
    [param: string]: string | number | string[] | number[];
};
export type BodyParams = {
    [param: string]: any;
};
export type Method = "GET" | "POST" | "PUT" | "PATCH" | "PURGE" | "DELETE" | "HEAD";
declare class API {
    private defaultHeaders;
    private defaultParams;
    private defaultBody;
    private url;
    constructor();
    setURL(url: string): void;
    setHeaders(headers: Headers): void;
    setBody(body: BodyParams): void;
    setGetParams(params: GetParams): void;
    /**
     * Perform a fetch request.
     * @example const response = await api.fetch<ExampleResponse>({ method: "POST", route: "/v1/user", body: { name: "Jon Smith" } });
     */
    fetch<T>(request: Request): Promise<T>;
    private buildBody;
    private buildRequestOptions;
    private attachGetParams;
}
declare const api: API;
export default api;
