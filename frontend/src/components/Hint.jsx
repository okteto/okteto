import React, { Component } from 'react';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';
import { CSSTransition } from 'react-transition-group';

import Icon from 'components/Icon';

import 'components/Hint.scss';

class Hint extends Component {
  constructor(props) {
    super(props);
    document.addEventListener('mousedown', this.handleOutsideClick);
  }

  componentWillUnmount() {
    document.removeEventListener('mousedown', this.handleOutsideClick);
  }

  @autobind
  handleOutsideClick(e) {
    if (this.props.closeOnBlur && this.hint && !this.hint.contains(e.target)) {
      this.close();
    }
  }

  close() {
    this.props.onClose && this.props.onClose();
  }

  render() {
    let positionStyles = {};
    if (this.props.positionX !== 'center') {
      if (this.props.positionX === 'left') { 
        positionStyles.left = `${this.props.offsetX}px`;
      } else if (this.props.positionX === 'right') { 
        positionStyles.right = `${this.props.offsetX}px`;
      }
    } else {
      positionStyles.left = '50%';
    }
    if (this.props.positionY === 'top') {
      positionStyles.bottom = `${this.props.offsetY}px`;
    } else {
      positionStyles.top = `${this.props.offsetY}px`;
    }
    return (
      <>
        {this.props.open &&
          <CSSTransition
            in={true}
            classNames="fade"
            appear={true}
            timeout={5000}>
            <div 
              className={`Hint ${this.props.className} ${this.props.positionY}`}
              style={{
                width: `${this.props.width}px`,
                ...positionStyles
              }}
              ref={e => { this.hint = e; }}
              onClick={e => e.stopPropagation()}>
              <div className="layout vertical">
                {!this.props.hideCloseButton &&
                  <div onClick={() => this.close()}>
                    <Icon 
                      className="hint-close-button" 
                      icon="close"
                      size="20"
                      color="black"
                    />
                  </div>
                }
                <div className="hint-content">
                  {this.props.children}
                </div>
              </div>
              <div className="arrow" />
            </div>
          </CSSTransition>
        }
      </>
    );
  }
}

Hint.defaultProps = {
  open: false,
  closeOnBlur: true,
  hideCloseButton: false,
  className: '',
  width: '220',
  offsetY: '40',
  offsetX: '0',
  positionY: 'bottom',
  positionX: 'center'
};

Hint.propTypes = {
  open: PropTypes.bool,
  closeOnBlur: PropTypes.bool,
  hideCloseButton: PropTypes.bool,
  onOpen: PropTypes.func,
  onClose: PropTypes.func,
  width: PropTypes.string,
  positionX: PropTypes.string,
  positionY: PropTypes.string,
  offsetX: PropTypes.string,
  offsetY: PropTypes.string,
  className: PropTypes.string,
  children: PropTypes.node.isRequired
};

export default Hint;
