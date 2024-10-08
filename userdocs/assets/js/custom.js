"use strict";


(function () {
    var Materio = {

        init: function () {
            this.materialize();
        },

        //init materialize framework features
        materialize: function () {

            $('.spy-toc').pushpin();
            $('.scrollspy').scrollSpy({
                scrollOffset: 0,
                getActiveElement: function (id) {
                    if (id === "your-service-definition") {
                        $('.spy-toc .table-of-contents a').addClass('sid');
                        return 'a[href="#' + id + '"]';
                    }
                    $('.spy-toc .table-of-contents a').removeClass('sid');
                    return 'a[href="#' + id + '"]';
                }
            });
            $(".button-collapse").sidenav();
            $('.dropdown-trigger').dropdown();
            $('.tabs').tabs();
            $('.materialboxed').materialbox();
        },
    }

    Materio.init();
})();
