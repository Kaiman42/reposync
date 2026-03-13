export namespace main {
	
	export class RepoInfo {
	    path: string;
	    name: string;
	    status: string;
	    size: string;
	    last_change: string;
	    relative_time: string;
	    remote_url: string;
	    commit_count: number;
	
	    static createFrom(source: any = {}) {
	        return new RepoInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.status = source["status"];
	        this.size = source["size"];
	        this.last_change = source["last_change"];
	        this.relative_time = source["relative_time"];
	        this.remote_url = source["remote_url"];
	        this.commit_count = source["commit_count"];
	    }
	}

}

