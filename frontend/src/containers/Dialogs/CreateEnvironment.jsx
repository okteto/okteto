import React, { Component } from 'react';
import * as ReactRedux from 'react-redux';
import PropTypes from 'prop-types';
import autobind from 'autobind-decorator';

import Button from 'components/Button';
import Modal from 'components/Modal';

import './CreateEnvironment.scss';

class CreateEnvironment extends Component {
  constructor(props) {
    super(props);
  }

  @autobind
  handleCloseClick() {
    this.close();
  }

  open() {
    this.dialog && this.dialog.open();
  }

  close() {
    this.dialog && this.dialog.close();
  }

  render() {
    return (
      <Modal 
        className="Create environment"
        ref={ref => this.dialog = ref} 
        title="New environment"
        width={500}
        offsetTop={8}>
        <div className="create-dialog-content layout vertical">
          {this.props.children}
          <div style={{ height: '20px' }} />
          <div className="layout horizontal-reverse center">
            <Button 
              color="grey"
              solid
              secondary
              onClick={this.handleCloseClick}>
              Close
            </Button>
          </div>
        </div>
      </Modal>
    );
  }
}

CreateEnvironment.propTypes = {
  children: PropTypes.node.isRequired,
  dispatch: PropTypes.func.isRequired
};

export default ReactRedux.connect(() => {
  return {};
}, null, null, { withRef: true })(CreateEnvironment);