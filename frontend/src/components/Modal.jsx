import React, { Component } from 'react';
import PropTypes from 'prop-types';
import Skylight from 'react-skylight';
import classnames from 'classnames';

import 'components/Modal.scss';

class Modal extends Component {
  constructor(props) {
    super(props);
  }

  open() {
    this.modal && this.modal.show();
  }

  close() {
    this.modal && this.modal.hide();
  }

  render() {
    const dialogStyles = {
      backgroundColor: 'white',
      color: '#171B22',
      minHeight: this.props.height ? `${this.props.height}px` : 'auto',
      width: `${this.props.width}px`,
      marginLeft: `-${this.props.width/2}px`,
      padding: '20px',
      zIndex: 99999999
    };

    const overlayStyles = {
      backgroundColor: 'rgba(0, 0, 0, 0.5)',
      zIndex: 99999999
    };

    const titleStyle = {
      fontSize: '20px',
      color: '#171B22',
      marginBottom: '32px'
    };

    const closeButtonStyle = {
      top: '15px',
      right: '20px',
      textDecoration: 'none'
    };

    return (
      <div className={classnames('Modal', this.props.className)}>
        <Skylight
          ref={ref => this.modal = ref}
          dialogStyles={dialogStyles}
          titleStyle={titleStyle}
          overlayStyles={overlayStyles}
          closeButtonStyle={closeButtonStyle}
          afterOpen={this.props.onOpen}
          afterClose={this.props.onClose}
          title={this.props.title}
          hideOnOverlayClicked>
          {this.props.children}
        </Skylight>
      </div>
    );
  }
}

Modal.defaultProps = {
  width: 200
};

Modal.propTypes = {
  title: PropTypes.string,
  height: PropTypes.number,
  width: PropTypes.number,
  onOpen: PropTypes.func,
  onClose: PropTypes.func,
  className: PropTypes.string,
  children: PropTypes.node.isRequired
};

export default Modal;
