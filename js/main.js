
$(document).ready(function () {
	$('.selall').change(function () {
		var checked = $(this).attr("checked");
		if (checked) {
			$('input[type="checkbox"]').attr('checked', checked);
		} else {
			$('input[type="checkbox"]').removeAttr('checked');
		}
	});

	var showerr = function (msg, id) {
		var a = $('#alert');
		if ($.alertsel) 
			a = $('#'+$.alertsel);
		if ($.cmodal) {
			a = $('<div>');
			$.cmodal.find('.modal-body').prepend(a);
		}
		if (a.length) {
			a.attr('style', 'width:400px');
			a.attr('class', 'alert alert-error myerr');
			a.html(
				msg +
				'	<button type="button" class="close" data-dismiss="alert">&times;</button>'
				);
		} else {
			alert(msg);
		}
	};

	var hideerr = function (msg) {
		$('.myerr').remove();
	};

	var arr = [];
	$('div[src^="/"]').each(function () {
		var d = $(this);
		var src = d.attr('src');
		$.get(src, function (data) {
			d.html(data);
		});
	});

	$('a').each(function () {
		var a = $(this);
		var hide = a.attr('hide');
		var show = a.attr('show');
		var reload = a.attr('reload');
		var post = a.attr('post');
		var modal = a.attr('modal');
		var hidem = a.attr('hidem');
		var formsel = a.attr('form');
		var formsel2 = a.attr('form2');
		var alertsel = a.attr('alert');
		var path = a.attr('path');
		var toggle = a.attr('toggle');
		var replace = a.attr('replace');
		var listadd = a.attr('listadd');
		var v = a.attr('v');
		if (hide || show || toggle || reload || post || modal || hidem) {
			a.click(function () {
				var jmp = function () {
					if (hide) 
						$('#'+hide).hide();
					if (hidem) {
						$.cmodal = null;
						$('#'+hidem).modal('hide');
					}
					if (reload)
						window.location.reload();
				};
				if (v)
					$.postv = v;
				if (formsel)
					$.formsel = formsel;
				if (formsel2)
					$.formsel2 = formsel2;
				if (alertsel)
					$.alertsel = alertsel;
				if (show)
					$('#'+show).show();
				if (toggle)
					$('#'+toggle).toggle();
				if (modal) {
					$.cmodal = $('#'+modal);
					$.cmodal.modal();
				}
				if (post) {
					var data = 'post='+post+'&';
					if ($.postv)
						data += $.postv+'&';
					var cform = a.closest('form');
					var form1 = $.formsel ? $('#'+$.formsel) : null;
					var form2 = $.formsel2 ? $('#'+$.formsel2) : null;
					if (cform.length) 
						data += cform.serialize()+'&';
					if (form1)
						data += form1.serialize()+'&';
					if (form2)
						data += form2.serialize()+'&';
					var vnode = a.closest('var');
					if (!path && vnode.attr('path') != '')
						path = vnode.attr('path');
					if (!path)
						path = '?';
					$.ajax({
						url: path,
						type: 'post',
						data: data,
						success: function(ret) {
							var obj = {};
							try {
								obj = jQuery.parseJSON(ret);
							} catch (e) {
							}
							console.log(data, ret);
							if (obj && obj.err) {
								hideerr();
								showerr(obj.err, obj.id);
							} else {
								hideerr();
								jmp();
							}
							$.postv = null;
							$.formsel = null;
							$.formsel2 = null;
						}
					});
				} else {
					jmp();
				}
			});
		}
	});
});

