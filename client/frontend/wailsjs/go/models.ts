export namespace main {
	
	export class Message {
	    id: number;
	    sender_id: number;
	    recipient_id: number;
	    content: string;
	    type: string;
	    file_url: string;
	    file_name: string;
	    // Go type: time
	    created_at: any;
	    sender_name?: string;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sender_id = source["sender_id"];
	        this.recipient_id = source["recipient_id"];
	        this.content = source["content"];
	        this.type = source["type"];
	        this.file_url = source["file_url"];
	        this.file_name = source["file_name"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.sender_name = source["sender_name"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

