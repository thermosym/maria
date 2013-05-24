
(function() {

	/* TODO
	 * [DONE] `hide selector` means hide closest selector
	 * [DONE] <div data="/xxx/xxx"> means fetch json data from /xxx/xxx
	 * [DONE] if div 'src' attr is empty, then using div.html() as template
	 * [DONE] code `r0 xxx xxx` means set reg varible. `r0/r1/r2...` can be accessed in `get/post`
	 * [DONE] <div dialog="xx xx"> means when $('#dialog') exec `ok` 'xx xx' will be executed
	 * [DONE] `show #dialog 123` <div dialog123="xx xx"> can also works
	 * [DONE] remove `onerr` stmt.
	 * [DONE] `href xx` jmp to xx
	 * [DONE] `post xx 'do=add'` => remove '' => `post xx do=add`
	 * [DONE] ajax POST/GET using json
	 * [DONE] `get/post` can only target url like `get /url dataA dataB`
	 * [DONE] solve problem: when rendering a div content, it has been rendered before
	 * 			using tpl cache method
	 * [DONE] add `render`. `render #xx dataA dataB`
	 * [DONE] combine `get` and `reload` as `load`. and can with parms like:
	 * 		 `load #xx /url/a` `load #xx dataA dataB`
	 * <a err="xx xx"> means when error exec `xx xx`; 
	 */

	var showerr = function (msg) {
		var a = $('#alert');
		if (a.length) {
			a.html('');
			var div = $('<div style="width:400px" class="alert alert-error">');
			div.html(
				msg + '	<button type="button" class="close" data-dismiss="alert">&times;</button></div>'
			);
			setTimeout(function () {
				div.fadeOut();
			}, 2000);
			a.prepend(div);
		} else {
			alert(msg);
		}
	};

	var hideerr = function () {
		$('.myerr').hide();
	};

	var render2 = function (dom, tpl, data) {
		var html = Mustache.render(tpl, data);
		dom.html(html);
		binda(dom);
		selall();
		dom.find('div[data]').each(function () {
			load($(this), null, null);
		});
	};

	var cachetpl = function (html) {
		var dom = $('<div style="display:none">');
		dom.html(html);
		dom.find('div[id]').each(function () {
			var id = $(this).attr('id');
			$.tplcache[id] = $(this).html();
			console.log('add tplcache', id);
		});
		dom.remove();
	};

	var render = function (dom, data) {
		var tpl = dom.attr('tpl');
		if (tpl) {
			$.get(tpl, function (_tpl) {
				console.log('render with', tpl, data);
				cachetpl(_tpl);
				render2(dom, _tpl, data);
			});
		} else {
			var _tpl;
			var domid = dom.attr('id');
			if (domid)
				_tpl = $.tplcache[domid];
			//if (_tpl)
			//	console.log('using tplcache', domid, _tpl);
			if (!_tpl)
				_tpl = dom.html();
			console.log('render', data);
			render2(dom, _tpl, data);
		}
	};
	
	var load = function (dom, url, query) {
		if (!url)
			url = dom.attr('data');
		if (!url)
			return ;
		if (!query)
			query = {};
		$.get(url, query, function (_data) {
			var data = parsejson(_data);
			render(dom, data);
		});
	};

	var selall = function () {
		$('.selall').change(function () {
			var checked = $(this).attr("checked");
			if (checked) {
				$('input[type="checkbox"]').attr('checked', checked);
			} else {
				$('input[type="checkbox"]').removeAttr('checked');
			}
		});
	};

	var getdom = function (st, v) {
		var op = v.substr(0,1);
		if (op == '#')
			return $(v);
		if (op == '$') {
			var a = st.a.closest('['+v.substr(1)+']');
			var vv = a.attr(v.substr(1));
			if (vv)
				return getdom(st, vv);
		}
		if (v == 'me')
			return st.a;
		return st.a.closest(v);
	};

	var form2json = function (dom) {
		var pair = dom.serializeArray();
		var json = {};
		for (var i in pair) {
			var k = pair[i].name;
			var v = pair[i].value;
			if (k && v) {
				if (!json[k]) 
					json[k] = [];
				json[k].push(v);
			}
		}
		return json;
	};

	var getdata = function (st, v) {
		if (v.substr(0,4) == 'form') {
			var form = st.a.closest('form');
			if (!form.length)
				form = st.a.parent().find('form');
			if (!form.length)
				return;
			if (v == 'form') 
				return form2json(form);
			if (v.substr(4,1) == '.') {
				var dom = form.find('[name="'+v.substr(5)+'"]');
				if (dom.length) { 
					var a = {};
					a[v.substr(5)] = dom.val();
					return a;
				}
				return;
			}
		}
		if (v.match(/=/)) {
			var a = v.split('=');
			if (a.length >= 2) {
				var j = {};
				j[a[0]] = a[1];
				return j;
			}
		}
		if (v.substr(0,1) == '#') 
			return form2json($(v).find('form'));
		if (v.match(/^r[0-9]+/))
			return $.regs[v];
		if (v == 'ret')
			return st.retobj;
	};

	var getdatas = function (st, arr) {
		var json = {};
		for (var i in arr) 
			jsonadd(json, getdata(st, arr[i]));
		return json;
	};

	var jsonadd = function (a, b) {
		for (var k in b) {
			a[k] = b[k];
		}
	}

	var parsejson = function (str) {
		var obj;
		try {
			obj = jQuery.parseJSON(str);
		} catch (e) {
			console.log('parsejson', e);
		}
		if (!obj)
			obj = {};
		return obj;
	};

	var func_load = function (st) {
		var dom = getdom(st.p.args[1]);
		if (!dom)
			return;
		var url;
		if (st.p.args[1][0] == '/') {
			url = st.p.args[1];
			st.p.args.shift();
		}
		var query;
		if (st.p.args.length > 1)
			query = getdatas(st, st.p.args.slice(1));
		load(dom, url, query);
	};

	var func_href = function (st) {
		window.location.href = st.p.args[1];
		window.location.reload();
	};

	var func_getpost = function (st) {
		var url = st.p.args[1];
		var method = st.p.op == 'get' ? 'GET' : 'POST';
		var data = getdatas(st, st.p.args.slice(2));
		var datastr = JSON.stringify(data);
		console.log('ajax', st.p.op, url, datastr);
		$.ajax({
			url: url,
			type: method,
			data: datastr,
		}).done(function (ret) {
			console.log('ajax ok', ret);
			st.ret = ret;
			st.retobj = parsejson(ret);
			if (st.retobj.err)
				st.err = st.retobj.err;
			st.cb(st);
		}).fail(function (ret) {
			console.log('ajax fail', ret.responseText);
			st.err = ret.responseText;
			st.cb(st);
		});
	};

	var func_hideshow = function (st) {
		var arr = st.p.args[1].split(',');
		var idx;
		if (st.p.args.length >= 3) 
			idx = st.p.args[2];
		for (var i in arr) {
			var dom = getdom(st, arr[i]);
			if (st.p.op == 'hide')
				dom.hide();
			if (st.p.op == 'show')
				dom.show();
			if (st.p.op == 'toggle')
				dom.toggle();
			if (idx)
				dom.attr('idx', idx);
		}
	};

	var func_render = function (st) {
		var dom = getdom(st, st.p.args[1]);
		if (!dom)
			return;
		var data = getdatas(st, st.p.args.slice(2));
		render(dom, data);
	};

	var func_r0 = function (st) {
		var data = getdatas(st, st.p.args.slice(1));
		$.regs[st.p.op] = data;
	};

	var func_ok = function (st) {
		var id = st.a.closest('div[id]').attr('id');
		if (!id)
			return;
		var idx = st.a.closest('div[id]').attr('idx');
		if (idx)
			id += idx;
		var doms = $('['+id+']');
		console.log('ok', id);
		doms.each(function () {
			var dom = $(this);
			parse(dom, id);
		});
	};

	var func_checkerr = function (st) {
		if (st.err) {
			showerr(st.err);
			return;
		}
		st.cb(st);
	};

	var func_confirm = function (st) {
		var str = st.p.args[1];
		str = str.substr(1,str.length-2);
		if (confirm(str))
			st.cb(st);
	};

	var exec = function (st) {
		if (st.i >= st.seq.length)
			return;
		st.p = st.seq[st.i++];
		console.log('exec', st.p.args, st.i+'/'+st.seq.length);
		if (st.p.op == 'ret')
			return;
		st.p.func(st);
		if (st.p.async)
			return;
		exec(st);
	};

	var parse = function (ele, attr) {
		var code = ele.attr(attr);
		if (!code) 
			return ;
		var codes = code.split(';');
		var st = {};
		st.seq = [];
		for (var i in codes) {
			var seq = {};
			seq.args = $.trim(codes[i]).split(' ');
			if (seq.args < 1)
				continue;
			console.log('parse', seq.args);

			if (seq.args[0].match(/^r[0-9]*/)) {
				if (seq.args.length < 2)
					continue;
				seq.func = func_r0;
			}
			switch (seq.args[0]) {
			case 'get':
			case 'post':
				if (seq.args.length < 2)
					continue;
				seq.func = func_getpost;
				seq.async = true;
				break;
			case 'hide':
			case 'show':
			case 'toggle':
				if (seq.args.length < 2)
					continue;
				seq.func = func_hideshow;
				break;
			case 'ret':
				break;
			case 'load':
				if (seq.args.length < 2)
					continue;
				seq.func = func_load;
				break;
			case 'ok':
				seq.func = func_ok;
				break;
			case 'checkerr':
				seq.func = func_checkerr;
				seq.async = true;
				break;
			case 'confirm':
				if (seq.args.length < 2)
					continue;
				seq.func = func_confirm;
				seq.async = true;
				break;
			case 'href':
				if (seq.args.length < 2)
					continue;
				seq.func = func_href;
				break;
			case 'render':
				if (seq.args.length < 2)
					continue;
				seq.func = func_render;
				break;
			}
			if (seq.func) {
				seq.op = seq.args[0];
				st.seq.push(seq);
			}
		}
		console.log('click', st.seq.length);
		st.a = ele;
		st.i = 0;
		st.cb = exec;
		st.okcb_n = 0;
		exec(st);
	};

	var binda = function (div) {
		div.find('a[do]').click(function () {
			var a = $(this);
			parse(a, 'do');
		});
	};

	$(document).ready(function () {
		$.regs = {};
		$.tplcache = {};

		var q = location.hash.substring(1);
		var a = q.split(',');
		for (var i = a.length; i < 4; i++)
			a.push('');
		if (a[0] == '') {
			a[0] = 'menu';
			a[1] = 'root';
		}
		console.log(location.hash);
		switch (a[0]) {
		case 'menu':
			$('#body').attr('data', '/menu/'+a[1]);
			$('#body').attr('tpl', '/tpl/menu1.html');
			break;
		case 'vfiles':
			$('#body').attr('data', '/vfiles');
			$('#body').attr('tpl', '/tpl/vlist1.html');
			break;
		case 'vlists':
			$('#body').attr('data', '/vlists');
			$('#body').attr('tpl', '/tpl/vlist2.html');
			break;
		default:
			return;
		}
		location.hash = '#'+a.join(',');

		$('#body').attr('tpl', '/tpl/vfileadd.html');
		load($('#body'));
	});
})();

