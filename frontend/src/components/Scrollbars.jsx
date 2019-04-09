import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator'
import css from 'dom-css';

import 'components/Scrollbars.scss';

class Scrollbars extends Component {
  constructor(props, ...rest) {
    super(props, ...rest);
    this.state = {
      scrollTop: 0,
      scrollHeight: 0,
      clientHeight: 0
    };
  }

  componentDidMount() {
    this.rootEl.parentElement.style.position = 'relative';
  }

  @autobind
  onScroll(event) {
    const shadowTopOpacity = 1 / 20 * Math.min(this.rootEl.scrollTop, 20);
    const bottomScrollTop = this.rootEl.scrollHeight - this.rootEl.clientHeight;
    const shadowBottomOpacity = 1 / 20 * 
      (bottomScrollTop - Math.max(this.rootEl.scrollTop, bottomScrollTop - 20));
    // css(this.shadowTop, { opacity: shadowTopOpacity });
    // css(this.shadowBottom, { opacity: shadowBottomOpacity });
  }

  render() {
    const shadowTopStyle = {
      position: 'absolute',
      top: this.props.topShadowOffset || 0,
      left: 0,
      right: 0,
      height: 10,
      background: 'linear-gradient(to bottom, rgba(0, 0, 0, 0.2) 0%, rgba(0, 0, 0, 0) 100%)'
    };
    const shadowBottomStyle = {
      position: 'absolute',
      bottom: 0,
      left: 0,
      right: 0,
      height: 10,
      background: 'linear-gradient(to top, rgba(0, 0, 0, 0.2) 0%, rgba(0, 0, 0, 0) 100%)'
    };
    return (
      <div className="Scrollbars" ref={(el) => this.rootEl = el } onScroll={this.onScroll}>
        {/* Temporarily Disabled */}
        {/* <div ref={(el) => this.shadowTop = el } style={shadowTopStyle}/> */}
        <div className="scrolling-content">
          {this.props.children}
        </div>
        {/* <div ref={(el) => this.shadowBottom = el } style={shadowBottomStyle} /> */}
      </div>
    );
  }
}

Scrollbars.propTypes = {
  style: PropTypes.object
};

export default Scrollbars;