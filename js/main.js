
(function() {

	var showerr = function (msg) {
		/*
		if (a.length) {
			a.attr('style', 'width:400px');
			a.attr('class', 'alert alert-error myerr');
			a.html(
				msg +
				'	<button type="button" class="close" data-dismiss="alert">&times;</button>'
				);
		}
	 	*/
		alert(msg);
	};

	var hideerr = function () {
		$('.myerr').hide();
	};
	
	var listadd = function (str, entry) {
		if (!str || str == '')
			return entry;
		var arr = str.split(',');
		for (var i in arr) {
			if (arr[i] == entry)
				return;
		}
		arr.push(entry);
		return arr.join(',');
	};

	var listdel = function (str, entry) {
		if (!str)
			return '';
		var newarr = [];
		var arr = str.split(',');
		for (var i in arr) {
			if (arr[i] != entry)
				newarr.push(arr[i]);
		}
		return newarr.join(',');
	};

	var reload = function (d, query) {
		var src = d.attr('src');
		if (!src)
			return ;
		src += '?';
		var list = d.attr('list');
		if (list) 
			src += 'list='+list+'&';
		if (query)
			src += query+'&';
		console.log('reload: ', src);
		$.get(src, function (data) {
			d.html(data);
			binddo(d);
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
		if (v == 'this')
			return st.a;
		if (v == 'div')
			return st.a.closest('div');
	};

	var geturl = function (st, v) {
		if (v.substr(0,1) == '/' || v.substr(0,1) == '?')
			return v;
		var dom = getdom(st, v);
		if (dom)
			return dom.attr('src');
	};

	var getdata = function (st, v) {
		if (v.substr(0,4) == 'form') {
			var form = st.a.closest('form');
			if (!form.length)
				form = st.a.parent().find('form');
			if (!form.length)
				return;
			if (v == 'form') 
				return form.serialize();
			if (v.substr(4,1) == '.') {
				var dom = form.find('[name="'+v.substr(5)+'"]');
				if (dom.length)
					return dom.val();
				return;
			}
		}
		if (v.substr(0,1) == "'")
			return v.substr(1,v.length-2);
		if (v.substr(0,1) == '#')
			return $(v).find('form').serialize();
	};

	var parsejson = function (str) {
		var obj;
		try {
			obj = jQuery.parseJSON(ret);
		} catch (e) {
		}
		if (!obj)
			obj = {};
		return obj;
	};

	var func_reload = function (st) {
		var d = st.p.args[1];
		if (d == 'page') {
			window.location.reload();
			return;
		}
		var dom = getdom(st, d);
		if (dom)
			reload(dom);
	};

	var func_getpost = function (st) {
		var data = '';
		for (var i = 2; i < st.p.args.length; i++) {
			var v = getdata(st, st.p.args[i]);
			if (v)
				data += v+'&';
		}
		if (st.data)
			data += st.data+'&';
		var url = geturl(st, st.p.args[1]);
		if (!url) {
			st.cb(st);
			return;
		}
		var dom = getdom(st, st.p.args[1]);
		var method = st.p.op == 'get' ? 'GET' : 'POST';
		//console.log(op, url, data, dom);
		$.ajax({
			url: url,
			type: method,
			data: data,
		}).done(function (ret) {
			console.log('ajax ok');
			st.ret = ret;
			st.retobj = parsejson(ret);
			if (st.retobj.err)
				st.err = retobj.err;
			if (dom && method == 'GET')
				dom.html(ret);
			st.cb(st);
		}).fail(function (ret) {
			st.err = ret.responseText;
			st.cb(st);
		});
	};

	var func_listdeladd = function (st) {
		var dom = getdom(st, st.p.args[1]);
		var val = getdata(st, st.p.args[2]);
		if (!val)
			return;
		switch (st.p.args[0]) {
		case 'listadd':
			dom.attr('list', listadd(dom.attr('list'), val));
			break;
		case 'listdel':
			dom.attr('list', listdel(dom.attr('list'), val));
			break;
		}
	};

	var func_hideshow = function (st) {
		var arr = st.p.args[1].split(',');
		for (var i in arr) {
			var dom = getdom(st, arr[i]);
			if (st.p.op == 'hide')
				dom.hide();
			if (st.p.op == 'show')
				dom.show();
		}
	};

	var func_ok = function (st) {
		var data = '';
		for (var i = 1; i < st.p.args.length; i++) {
			var val = getdata(st, st.p.args[i]);
			if (val)
				data += val+'&';
		}
		console.log('onok', data);
		if ($.onok) 
			$.onok(data);
	};

	var func_onok = function (st) {
		$.onok = function (data) {
			st.data = data;
			st.cb(st);
		};
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
		if (st.p.okcb) {
			add_okcb(st, st.p.okcb);
		} else if (st.p.conderr) {
			if (st.err) 
				st.p.func(st);
		} else
			st.p.func(st);
		if (st.p.async)
			return;
		exec(st);
	};

	var binddo = function (div) {
		div.find('a').click(function () {
			var a = $(this);
			var code = a.attr('do');
			if (!code) 
				return ;
			var onok;
			var codes = code.split(';');
			var st = {};
			st.seq = [];
			for (var i in codes) {
				var seq = {};
				seq.args = $.trim(codes[i]).split(' ');
				if (seq.args < 1)
					continue;
				console.log('parse', seq.args, codes[i].split(' '));
				if (seq.args[0] == 'onerr') {
					if (seq.args.length < 2)
						continue;
					seq.conderr = true;
					seq.args.shift();
				}
				switch (seq.args[0]) {
				case 'get':
				case 'post':
					if (seq.args.length < 2)
						continue;
					seq.func = func_getpost;
					seq.async = true;
					break;
				case 'listadd':
				case 'listdel':
					if (seq.args.length < 3)
						continue;
					seq.func = func_listdeladd; 
					break;
				case 'hide':
				case 'show':
					if (seq.args.length < 2)
						continue;
					seq.func = func_hideshow;
					break;
				case 'ret':
					break;
				case 'reload':
					if (seq.args.length < 2)
						continue;
					seq.func = func_reload;
					break;
				case 'ok':
					seq.func = func_ok;
					break;
				case 'onok':
					seq.func = func_onok;
					seq.async = true;
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
				default:
					continue;
				}
				seq.op = seq.args[0];
				st.seq.push(seq);
			}
			console.log('click', st.seq.length);
			st.a = a;
			st.i = 0;
			st.cb = exec;
			st.okcb_n = 0;
			exec(st);
		});
	};

	$(document).ready(function () {
		binddo($('body'));
		$('div[src]').each(function () {
			reload($(this));
		});
		selall();
	});

})();

